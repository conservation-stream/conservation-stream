mod catalog;
mod jit;

use std::path::PathBuf;

use catalog::{FilteredCatalog, Rendition};
use clap::Parser;
use hang::moq_net;
use jit::{JitPublisher, SourceConfig, VideoCodec};
use moq_mux::container::fmp4;
use url::Url;

#[derive(Parser, Clone)]
#[command(version)]
struct Cli {
    #[command(flatten)]
    log: moq_native::Log,

    /// The MoQ client configuration.
    #[command(flatten)]
    config: moq_native::ClientConfig,

    /// The URL of the MoQ relay.
    #[arg(long)]
    url: Url,

    /// The name of the broadcast to publish.
    #[arg(long, alias = "name")]
    broadcast: String,

    /// RTSP source to encode just in time. If omitted, fMP4 is read from stdin.
    #[arg(long = "source-rtsp")]
    source_rtsp: Option<String>,

    /// Video codec to publish when reading RTSP.
    #[arg(long = "video-codec", default_value = "h264")]
    video_codec: VideoCodec,

    /// Configured rendition as name:WIDTHxHEIGHT:BITRATE, passthrough, or name:passthrough.
    #[arg(long = "rendition")]
    renditions: Vec<Rendition>,

    /// Limit the catalog to these configured rendition names.
    #[arg(long = "advertise-rendition")]
    advertise_renditions: Vec<String>,

    /// JSON control file for live catalog advertising changes.
    ///
    /// Shape: {"advertise":["360p"]}. Omitting the field advertises all renditions.
    #[arg(long = "catalog-control")]
    catalog_control: Option<PathBuf>,
}

struct Publish {
    broadcast: moq_net::BroadcastProducer,
    source: PublishSource,
}

enum PublishSource {
    StdinFmp4 {
        source: Box<fmp4::Import>,
        scratch_catalog: moq_mux::catalog::Producer,
        filtered_catalog: FilteredCatalog,
    },
    Jit(JitPublisher),
}

impl Publish {
    fn new(
        renditions: Vec<Rendition>,
        advertise_renditions: Vec<String>,
        source_rtsp: Option<String>,
        video_codec: VideoCodec,
    ) -> anyhow::Result<Self> {
        let mut broadcast = moq_net::Broadcast::new().produce();
        let filtered_catalog =
            FilteredCatalog::new(&mut broadcast, renditions.clone(), advertise_renditions)?;

        let source = if let Some(rtsp_url) = source_rtsp {
            PublishSource::Jit(JitPublisher::new(
                &mut broadcast,
                filtered_catalog,
                SourceConfig {
                    rtsp_url,
                    video_codec,
                    renditions,
                },
            )?)
        } else {
            let mut scratch_broadcast = moq_net::Broadcast::new().produce();
            let scratch_catalog = moq_mux::catalog::Producer::new(&mut scratch_broadcast)?;
            let source = fmp4::Import::new(broadcast.clone(), scratch_catalog.clone());
            PublishSource::StdinFmp4 {
                source: Box::new(source),
                scratch_catalog,
                filtered_catalog,
            }
        };

        Ok(Self { broadcast, source })
    }

    fn consume(&self) -> moq_net::BroadcastConsumer {
        self.broadcast.consume()
    }

    fn catalog(&self) -> FilteredCatalog {
        match &self.source {
            PublishSource::StdinFmp4 {
                filtered_catalog, ..
            } => filtered_catalog.clone(),
            PublishSource::Jit(jit) => jit.catalog(),
        }
    }

    async fn run(mut self) -> anyhow::Result<()> {
        match self.source {
            PublishSource::StdinFmp4 {
                ref mut source,
                ref scratch_catalog,
                ref filtered_catalog,
            } => {
                let mut stdin = tokio::io::stdin();
                let mut buffer = bytes::BytesMut::new();

                loop {
                    let n = tokio::io::AsyncReadExt::read_buf(&mut stdin, &mut buffer).await?;
                    if n == 0 {
                        return Ok(());
                    }
                    source.decode(&mut buffer)?;

                    if source.is_initialized() {
                        filtered_catalog.sync_from(scratch_catalog.snapshot())?;
                    }
                }
            }
            PublishSource::Jit(jit) => jit.run().await,
        }
    }
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    rustls::crypto::aws_lc_rs::default_provider()
        .install_default()
        .expect("failed to install default crypto provider");

    let cli = Cli::parse();
    cli.log.init()?;

    anyhow::ensure!(
        !cli.renditions.is_empty(),
        "at least one --rendition is required"
    );

    let mut advertise_renditions = cli.advertise_renditions;
    if let Some(path) = &cli.catalog_control {
        match catalog::read_control_file(path).await {
            Ok(Some(active)) => advertise_renditions = active.into_iter().collect(),
            Ok(None) => advertise_renditions.clear(),
            Err(err)
                if err
                    .downcast_ref::<std::io::Error>()
                    .is_some_and(|io| io.kind() == std::io::ErrorKind::NotFound) => {}
            Err(err) => return Err(err),
        }
    }

    let publish = Publish::new(
        cli.renditions,
        advertise_renditions,
        cli.source_rtsp,
        cli.video_codec,
    )?;

    if let Some(path) = cli.catalog_control {
        let catalog = publish.catalog();
        tokio::spawn(async move {
            if let Err(err) = catalog::watch_control_file(catalog, path).await {
                tracing::error!(?err, "catalog control watcher exited");
            }
        });
    }

    let client = cli.config.init()?;
    run_client(client, cli.url, cli.broadcast, publish).await
}

async fn run_client(
    client: moq_native::Client,
    url: Url,
    name: String,
    publish: Publish,
) -> anyhow::Result<()> {
    let origin = moq_net::Origin::random().produce();
    origin.publish_broadcast(&name, publish.consume());

    tracing::info!(%url, %name, "connecting");

    let reconnect = client.with_publish(origin.consume()).reconnect(url);

    tokio::select! {
        res = publish.run() => res,
        res = reconnect.closed() => Ok(res?),
        _ = tokio::signal::ctrl_c() => Ok(()),
    }
}
