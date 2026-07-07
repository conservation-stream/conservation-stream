use std::{
    pin::Pin,
    process::Stdio,
    sync::{Arc, Mutex},
    time::{Duration, Instant},
};

use anyhow::Context;
use bytes::{BufMut, Bytes, BytesMut};
use futures::{Stream, StreamExt};
use hang::moq_net;
use tokio::{io::AsyncReadExt, process::Command};
use url::Url;

use crate::catalog::{FilteredCatalog, Rendition};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum VideoCodec {
    H264,
    H265,
}

impl std::str::FromStr for VideoCodec {
    type Err = anyhow::Error;

    fn from_str(raw: &str) -> Result<Self, Self::Err> {
        match raw.trim().to_ascii_lowercase().as_str() {
            "h264" | "avc" | "avc3" => Ok(Self::H264),
            "h265" | "hevc" | "hev1" => Ok(Self::H265),
            _ => anyhow::bail!("video codec must be h264 or h265"),
        }
    }
}

impl std::fmt::Display for VideoCodec {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::H264 => write!(f, "h264"),
            Self::H265 => write!(f, "h265"),
        }
    }
}

#[derive(Clone)]
pub struct SourceConfig {
    pub sources: Vec<RtspRenditionSource>,
}

#[derive(Clone)]
pub struct RtspRenditionSource {
    pub rtsp_url: String,
    pub video_codec: VideoCodec,
    pub rendition: Rendition,
}

pub struct JitPublisher {
    catalog: FilteredCatalog,
    clock: MediaClock,
    tracks: BroadcastTracks,
    source: SourceConfig,
}

impl JitPublisher {
    pub fn new(
        broadcast: &moq_net::BroadcastProducer,
        catalog: FilteredCatalog,
        source: SourceConfig,
    ) -> anyhow::Result<Self> {
        Ok(Self {
            catalog,
            clock: MediaClock::new(),
            tracks: BroadcastTracks::new(broadcast),
            source,
        })
    }

    pub fn catalog(&self) -> FilteredCatalog {
        self.catalog.clone()
    }

    pub async fn run(self) -> anyhow::Result<()> {
        for source in self.source.sources {
            let catalog = self.catalog.clone();
            let clock = self.clock;
            let tracks = self.tracks.clone();

            tokio::spawn(async move {
                if let Err(err) = run_rendition_worker(source, tracks, catalog, clock).await {
                    tracing::error!(?err, "rendition worker exited");
                }
            });
        }

        std::future::pending::<()>().await;
        Ok(())
    }
}

async fn run_rendition_worker(
    source: RtspRenditionSource,
    tracks: BroadcastTracks,
    catalog: FilteredCatalog,
    clock: MediaClock,
) -> anyhow::Result<()> {
    loop {
        wait_until_active(&source.rendition.name, &catalog).await;

        let exit = if source.rendition.passthrough {
            run_rtsp_passthrough_while_active(&source, &tracks, &catalog).await
        } else {
            run_ffmpeg_encoder_while_active(&source, &tracks, &catalog, clock).await
        };

        match exit {
            Ok(EncoderExit::Inactive) => {
                catalog.remove_label(&source.rendition.name)?;
            }
            Err(err) => {
                tracing::warn!(rendition = %source.rendition.name, ?err, "rendition worker stopped");
            }
        }
    }
}

async fn wait_until_active(label: &str, catalog: &FilteredCatalog) {
    loop {
        if catalog.is_active(label) {
            return;
        }

        tokio::time::sleep(Duration::from_millis(100)).await;
    }
}

struct RenditionSession {
    import: VideoImport,
    scratch_catalog: moq_mux::catalog::Producer,
    track: moq_net::TrackProducer,
}

enum EncoderExit {
    Inactive,
}

#[derive(Clone)]
struct BroadcastTracks {
    broadcast: Arc<Mutex<moq_net::BroadcastProducer>>,
}

impl BroadcastTracks {
    fn new(broadcast: &moq_net::BroadcastProducer) -> Self {
        Self {
            broadcast: Arc::new(Mutex::new(broadcast.clone())),
        }
    }

    fn video_import(
        &self,
        codec: VideoCodec,
        catalog: moq_mux::catalog::Producer,
    ) -> anyhow::Result<VideoImport> {
        let broadcast = self
            .broadcast
            .lock()
            .expect("broadcast track lock poisoned");
        VideoImport::new(codec, broadcast.clone(), catalog)
    }
}

enum VideoImport {
    H264(moq_mux::codec::h264::Import),
    H265(moq_mux::codec::h265::Import),
}

impl VideoImport {
    fn new(
        codec: VideoCodec,
        broadcast: moq_net::BroadcastProducer,
        catalog: moq_mux::catalog::Producer,
    ) -> anyhow::Result<Self> {
        Ok(match codec {
            VideoCodec::H264 => Self::H264(
                moq_mux::codec::h264::Import::new(broadcast, catalog)
                    .with_mode(moq_mux::codec::h264::Mode::Avc3)?,
            ),
            VideoCodec::H265 => Self::H265(moq_mux::codec::h265::Import::new(broadcast, catalog)),
        })
    }

    fn initialize(&mut self, init: &mut BytesMut) -> anyhow::Result<()> {
        match self {
            Self::H264(import) => import.initialize(init),
            Self::H265(import) => import.initialize(init),
        }
    }

    fn track(&self) -> anyhow::Result<&moq_net::TrackProducer> {
        match self {
            Self::H264(import) => import
                .track()
                .ok_or_else(|| anyhow::anyhow!("H.264 importer did not create a track")),
            Self::H265(import) => import.track(),
        }
    }

    fn decode_frame(
        &mut self,
        frame: &mut BytesMut,
        timestamp: Option<moq_mux::container::Timestamp>,
    ) -> anyhow::Result<()> {
        match self {
            Self::H264(import) => import.decode_frame(frame, timestamp),
            Self::H265(import) => import.decode_frame(frame, timestamp),
        }
    }
}

async fn run_rtsp_passthrough_while_active(
    source: &RtspRenditionSource,
    tracks: &BroadcastTracks,
    catalog: &FilteredCatalog,
) -> anyhow::Result<EncoderExit> {
    let mut rtsp = open_rtsp_frame_stream(&source.rtsp_url, Some(source.video_codec)).await?;
    let mut session = seed_rendition_from_rtsp(
        &mut rtsp,
        source.video_codec,
        &source.rendition,
        tracks,
        catalog.clone(),
    )
    .await?;

    tracing::info!(
        rendition = %source.rendition.name,
        track = %session.track.name,
        "starting active RTSP/RTP rendition reader"
    );

    let mut active_check = tokio::time::interval(Duration::from_millis(100));

    loop {
        tokio::select! {
            frame = rtsp.next_video_frame() => {
                let Some(mut frame) = frame? else {
                    anyhow::bail!("RTSP source ended");
                };
                session.import.decode_frame(&mut frame.data, Some(frame.timestamp))?;
                catalog.upsert_video_catalog(&source.rendition.name, session.scratch_catalog.snapshot())?;
            }
            _ = active_check.tick() => {
                if !catalog.is_active(&source.rendition.name) {
                    tracing::info!(
                        rendition = %source.rendition.name,
                        track = %session.track.name,
                        "stopping unadvertised RTSP/RTP rendition reader"
                    );
                    return Ok(EncoderExit::Inactive);
                }
            }
        }
    }
}

async fn run_ffmpeg_encoder_while_active(
    source: &RtspRenditionSource,
    tracks: &BroadcastTracks,
    catalog: &FilteredCatalog,
    clock: MediaClock,
) -> anyhow::Result<EncoderExit> {
    let mut session = seed_rendition_with_ffmpeg(
        &source.rtsp_url,
        source.video_codec,
        &source.rendition,
        tracks,
        catalog.clone(),
    )
    .await?;

    tracing::info!(
        rendition = %source.rendition.name,
        track = %session.track.name,
        "starting active rendition encoder"
    );

    run_encoder_while_active(
        &source.rtsp_url,
        source.video_codec,
        &source.rendition,
        catalog,
        &mut session,
        clock,
    )
    .await
}

async fn seed_rendition_from_rtsp(
    rtsp: &mut RtspFrameStream,
    video_codec: VideoCodec,
    rendition: &Rendition,
    tracks: &BroadcastTracks,
    catalog: FilteredCatalog,
) -> anyhow::Result<RenditionSession> {
    tracing::info!(rendition = %rendition.name, "seeding rendition catalog from RTSP/RTP");

    let mut scratch_broadcast = moq_net::Broadcast::new().produce();
    let scratch_catalog = moq_mux::catalog::Producer::new(&mut scratch_broadcast)?;
    let mut import = tracks.video_import(video_codec, scratch_catalog.clone())?;
    let mut seed = ParameterSetSeed::new(video_codec);

    let mut seed_frame = tokio::time::timeout(Duration::from_secs(15), async {
        loop {
            let Some(frame) = rtsp.next_video_frame().await? else {
                anyhow::bail!("RTSP source ended before {}", seed.description());
            };
            let mut scan = frame.data.clone();
            let mut nals = moq_mux::codec::annexb::NalIterator::new(&mut scan);
            while let Some(nal) = nals.next().transpose()? {
                if seed.observe(&nal)? {
                    return Ok::<_, anyhow::Error>(frame);
                }
            }
        }
    })
    .await
    .map_err(|_| anyhow::anyhow!("timed out waiting for {} seed", seed.description()))??;

    import.initialize(seed.init_mut())?;
    let track = import.track()?.clone();
    catalog.upsert_video_catalog(&rendition.name, scratch_catalog.snapshot())?;
    import.decode_frame(&mut seed_frame.data, Some(seed_frame.timestamp))?;
    catalog.upsert_video_catalog(&rendition.name, scratch_catalog.snapshot())?;

    tracing::info!(
        rendition = %rendition.name,
        track = %track.name,
        "seeded rendition catalog from RTSP/RTP"
    );

    Ok(RenditionSession {
        import,
        scratch_catalog,
        track,
    })
}

async fn seed_rendition_with_ffmpeg(
    rtsp_url: &str,
    video_codec: VideoCodec,
    rendition: &Rendition,
    tracks: &BroadcastTracks,
    catalog: FilteredCatalog,
) -> anyhow::Result<RenditionSession> {
    tracing::info!(rendition = %rendition.name, "seeding rendition catalog");

    let mut scratch_broadcast = moq_net::Broadcast::new().produce();
    let scratch_catalog = moq_mux::catalog::Producer::new(&mut scratch_broadcast)?;
    let mut import = tracks.video_import(video_codec, scratch_catalog.clone())?;

    let mut child = spawn_ffmpeg(rtsp_url, rendition, video_codec)?;
    let mut stdout = child
        .stdout
        .take()
        .ok_or_else(|| anyhow::anyhow!("ffmpeg stdout was not piped"))?;

    let mut buffer = BytesMut::new();
    let mut seed = ParameterSetSeed::new(video_codec);

    let wait_for_seed = async {
        loop {
            let n = stdout.read_buf(&mut buffer).await?;
            if n == 0 {
                let status = child.wait().await?;
                anyhow::bail!(
                    "ffmpeg exited before {} seed with {status}",
                    seed.description()
                );
            }

            let mut nals = moq_mux::codec::annexb::NalIterator::new(&mut buffer);
            while let Some(nal) = nals.next().transpose()? {
                if seed.observe(&nal)? {
                    return Ok::<_, anyhow::Error>(());
                }
            }
        }
    };

    tokio::time::timeout(Duration::from_secs(15), wait_for_seed)
        .await
        .map_err(|_| anyhow::anyhow!("timed out waiting for {} seed", seed.description()))??;

    let _ = child.start_kill();
    let _ = child.wait().await;

    import.initialize(seed.init_mut())?;
    let track = import.track()?.clone();
    catalog.upsert_video_catalog(&rendition.name, scratch_catalog.snapshot())?;
    tracing::info!(
        rendition = %rendition.name,
        track = %track.name,
        "seeded rendition catalog"
    );

    Ok(RenditionSession {
        import,
        scratch_catalog,
        track,
    })
}

async fn run_encoder_while_active(
    rtsp_url: &str,
    video_codec: VideoCodec,
    rendition: &Rendition,
    catalog: &FilteredCatalog,
    session: &mut RenditionSession,
    clock: MediaClock,
) -> anyhow::Result<EncoderExit> {
    let mut child = spawn_ffmpeg(rtsp_url, rendition, video_codec)?;
    let mut stdout = child
        .stdout
        .take()
        .ok_or_else(|| anyhow::anyhow!("ffmpeg stdout was not piped"))?;
    let mut buffer = BytesMut::new();
    let mut access_units = AnnexBAccessUnits::new(video_codec, clock);
    let mut active_check = tokio::time::interval(Duration::from_millis(100));

    loop {
        tokio::select! {
            n = stdout.read_buf(&mut buffer) => {
                let n = n?;
                if n == 0 {
                    let status = child.wait().await?;
                    anyhow::bail!("ffmpeg exited with {status}");
                }
                if access_units.decode(&mut session.import, &mut buffer)? {
                    catalog.upsert_video_catalog(&rendition.name, session.scratch_catalog.snapshot())?;
                }
            }
            _ = active_check.tick() => {
                if !catalog.is_active(&rendition.name) {
                    tracing::info!(
                        rendition = %rendition.name,
                        track = %session.track.name,
                        "stopping unadvertised rendition encoder"
                    );
                    let _ = child.start_kill();
                    let _ = child.wait().await;
                    return Ok(EncoderExit::Inactive);
                }
            }
        }
    }
}

type DemuxedStream =
    Pin<Box<dyn Stream<Item = Result<retina::codec::CodecItem, retina::Error>> + Send>>;

struct RtspFrame {
    data: BytesMut,
    timestamp: moq_mux::container::Timestamp,
}

struct RtspFrameStream {
    stream_id: usize,
    frames: DemuxedStream,
}

impl RtspFrameStream {
    async fn next_video_frame(&mut self) -> anyhow::Result<Option<RtspFrame>> {
        while let Some(item) = self.frames.next().await {
            match item? {
                retina::codec::CodecItem::VideoFrame(frame)
                    if frame.stream_id() == self.stream_id =>
                {
                    return Ok(Some(RtspFrame {
                        timestamp: retina_timestamp_to_moq(frame.timestamp())?,
                        data: BytesMut::from(frame.data()),
                    }));
                }
                _ => {}
            }
        }

        Ok(None)
    }
}

async fn open_rtsp_frame_stream(
    raw_url: &str,
    expected_codec: Option<VideoCodec>,
) -> anyhow::Result<RtspFrameStream> {
    let (url, credentials) = rtsp_url_and_credentials(raw_url)?;
    let mut session = retina::client::Session::describe(
        url,
        retina::client::SessionOptions::default()
            .creds(credentials)
            .user_agent("cstream-moq-publisher".to_string()),
    )
    .await?;

    let stream_id = session
        .streams()
        .iter()
        .position(|stream| {
            stream.media() == "video"
                && video_codec_from_rtsp_encoding(stream.encoding_name())
                    .is_some_and(|codec| expected_codec.is_none_or(|expected| expected == codec))
        })
        .context("RTSP source did not offer the expected H.264/H.265 video stream")?;

    session
        .setup(
            stream_id,
            retina::client::SetupOptions::default()
                .transport(retina::client::Transport::Tcp(
                    retina::client::TcpTransportOptions::default(),
                ))
                .frame_format(retina::codec::FrameFormat::SIMPLE),
        )
        .await?;

    let demuxed = session
        .play(retina::client::PlayOptions::default())
        .await?
        .demuxed()?;

    Ok(RtspFrameStream {
        stream_id,
        frames: Box::pin(demuxed),
    })
}

fn rtsp_url_and_credentials(
    raw_url: &str,
) -> anyhow::Result<(Url, Option<retina::client::Credentials>)> {
    let mut url = Url::parse(raw_url).context("parse RTSP URL")?;
    let username = url.username().to_string();
    if username.is_empty() {
        return Ok((url, None));
    }

    let password = url.password().unwrap_or_default().to_string();
    url.set_username("")
        .map_err(|_| anyhow::anyhow!("failed to strip username from RTSP URL"))?;
    url.set_password(None)
        .map_err(|_| anyhow::anyhow!("failed to strip password from RTSP URL"))?;

    Ok((
        url,
        Some(retina::client::Credentials { username, password }),
    ))
}

fn retina_timestamp_to_moq(
    timestamp: retina::Timestamp,
) -> anyhow::Result<moq_mux::container::Timestamp> {
    let elapsed = timestamp.elapsed();
    anyhow::ensure!(elapsed >= 0, "RTSP frame timestamp was before stream start");
    let micros = (elapsed as u128 * 1_000_000) / timestamp.clock_rate().get() as u128;
    Ok(moq_mux::container::Timestamp::from_micros(
        u64::try_from(micros).context("RTSP timestamp overflow")?,
    )?)
}

struct AnnexBAccessUnits {
    codec: VideoCodec,
    current: BytesMut,
    contains_slice: bool,
    clock: FrameClock,
}

impl AnnexBAccessUnits {
    fn new(codec: VideoCodec, clock: MediaClock) -> Self {
        Self {
            codec,
            current: BytesMut::new(),
            contains_slice: false,
            clock: FrameClock::new(clock),
        }
    }

    fn decode(&mut self, import: &mut VideoImport, buffer: &mut BytesMut) -> anyhow::Result<bool> {
        let mut wrote_frame = false;
        let mut nals = moq_mux::codec::annexb::NalIterator::new(buffer);

        while let Some(nal) = nals.next().transpose()? {
            wrote_frame |= self.push(import, nal)?;
        }

        Ok(wrote_frame)
    }

    fn push(&mut self, import: &mut VideoImport, nal: Bytes) -> anyhow::Result<bool> {
        let starts_access_unit = starts_access_unit(self.codec, &nal)?;
        let mut wrote_frame = false;

        if self.contains_slice && starts_access_unit {
            self.flush(import)?;
            wrote_frame = true;
        }

        self.current.put_slice(&moq_mux::codec::annexb::START_CODE);
        self.current.put_slice(&nal);

        if is_slice_nal(self.codec, &nal)? {
            self.contains_slice = true;
        }

        Ok(wrote_frame)
    }

    fn flush(&mut self, import: &mut VideoImport) -> anyhow::Result<()> {
        let mut frame = std::mem::take(&mut self.current);
        self.contains_slice = false;
        import.decode_frame(&mut frame, Some(self.clock.next_timestamp()?))
    }
}

#[derive(Clone, Copy)]
struct MediaClock {
    zero: Instant,
}

impl MediaClock {
    fn new() -> Self {
        Self {
            zero: Instant::now(),
        }
    }

    fn elapsed_micros(self) -> u64 {
        self.zero.elapsed().as_micros() as u64
    }
}

struct FrameClock {
    media: MediaClock,
    last_micros: Option<u64>,
}

impl FrameClock {
    const MIN_FRAME_MICROS: u64 = 1_000_000 / 60;

    fn new(media: MediaClock) -> Self {
        Self {
            media,
            last_micros: None,
        }
    }

    fn next_timestamp(&mut self) -> anyhow::Result<moq_mux::container::Timestamp> {
        Ok(moq_mux::container::Timestamp::from_micros(
            self.next_micros(),
        )?)
    }

    fn next_micros(&mut self) -> u64 {
        let elapsed = self.media.elapsed_micros();
        let micros = match self.last_micros {
            Some(last) if elapsed.saturating_sub(last) < Self::MIN_FRAME_MICROS => {
                last + Self::MIN_FRAME_MICROS
            }
            _ => elapsed,
        };

        self.last_micros = Some(micros);
        micros
    }
}

struct ParameterSetSeed {
    codec: VideoCodec,
    init: BytesMut,
    saw_vps: bool,
    saw_sps: bool,
    saw_pps: bool,
}

impl ParameterSetSeed {
    fn new(codec: VideoCodec) -> Self {
        Self {
            codec,
            init: BytesMut::new(),
            saw_vps: false,
            saw_sps: false,
            saw_pps: false,
        }
    }

    fn observe(&mut self, nal: &[u8]) -> anyhow::Result<bool> {
        if self.is_parameter_set(nal)? {
            self.init.put_slice(&moq_mux::codec::annexb::START_CODE);
            self.init.put_slice(nal);
        }

        Ok(match self.codec {
            VideoCodec::H264 => self.saw_sps && self.saw_pps,
            VideoCodec::H265 => self.saw_vps && self.saw_sps && self.saw_pps,
        })
    }

    fn is_parameter_set(&mut self, nal: &[u8]) -> anyhow::Result<bool> {
        match self.codec {
            VideoCodec::H264 => {
                let kind = h264_nal_type(nal)?;
                self.saw_sps |= kind == 7;
                self.saw_pps |= kind == 8;
                Ok(kind == 7 || kind == 8)
            }
            VideoCodec::H265 => {
                let kind = h265_nal_type(nal)?;
                self.saw_vps |= kind == 32;
                self.saw_sps |= kind == 33;
                self.saw_pps |= kind == 34;
                Ok(matches!(kind, 32..=34))
            }
        }
    }

    fn init_mut(&mut self) -> &mut BytesMut {
        &mut self.init
    }

    fn description(&self) -> &'static str {
        match self.codec {
            VideoCodec::H264 => "SPS/PPS",
            VideoCodec::H265 => "VPS/SPS/PPS",
        }
    }
}

fn starts_access_unit(codec: VideoCodec, nal: &[u8]) -> anyhow::Result<bool> {
    Ok(match codec {
        VideoCodec::H264 => match h264_nal_type(nal)? {
            1..=5 => nal.get(1).is_some_and(|header| header & 0x80 != 0),
            6..=9 => true,
            _ => false,
        },
        VideoCodec::H265 => match h265_nal_type(nal)? {
            0..=31 => nal.get(2).is_some_and(|header| header & 0x80 != 0),
            32..=40 => true,
            _ => false,
        },
    })
}

fn is_slice_nal(codec: VideoCodec, nal: &[u8]) -> anyhow::Result<bool> {
    Ok(match codec {
        VideoCodec::H264 => matches!(h264_nal_type(nal)?, 1..=5),
        VideoCodec::H265 => matches!(h265_nal_type(nal)?, 0..=31),
    })
}

fn h264_nal_type(nal: &[u8]) -> anyhow::Result<u8> {
    let Some(kind) = nal.first().map(|header| header & 0x1f) else {
        anyhow::bail!("H.264 NAL unit is too short");
    };
    Ok(kind)
}

fn h265_nal_type(nal: &[u8]) -> anyhow::Result<u8> {
    anyhow::ensure!(nal.len() >= 2, "H.265 NAL unit is too short");
    Ok((nal[0] >> 1) & 0x3f)
}

fn spawn_ffmpeg(
    rtsp_url: &str,
    rendition: &Rendition,
    video_codec: VideoCodec,
) -> anyhow::Result<tokio::process::Child> {
    let loglevel = if rendition.passthrough {
        "error"
    } else {
        "warning"
    };

    let mut args = vec![
        "-hide_banner".to_string(),
        "-loglevel".to_string(),
        loglevel.to_string(),
        "-nostdin".to_string(),
    ];

    args.extend(input_latency_args(video_codec, rendition.passthrough));

    args.extend([
        "-rtsp_transport".to_string(),
        "tcp".to_string(),
        "-i".to_string(),
        rtsp_url.to_string(),
        "-an".to_string(),
        "-map".to_string(),
        "0:v:0".to_string(),
    ]);

    if rendition.passthrough {
        args.extend([
            "-c:v".to_string(),
            "copy".to_string(),
            "-bsf:v".to_string(),
            passthrough_bitstream_filter(video_codec).to_string(),
        ]);
    } else {
        args.extend([
            "-vf".to_string(),
            format!("scale={}:{}", rendition.width, rendition.height),
        ]);
        args.extend(encode_args(rendition, video_codec));
    }

    args.extend([
        "-map_metadata".to_string(),
        "-1".to_string(),
        "-f".to_string(),
        raw_muxer(video_codec).to_string(),
        "-".to_string(),
    ]);

    Ok(Command::new("ffmpeg")
        .args(args)
        .stdout(Stdio::piped())
        .stderr(Stdio::inherit())
        .spawn()?)
}

fn input_latency_args(video_codec: VideoCodec, passthrough: bool) -> Vec<String> {
    match video_codec {
        VideoCodec::H264 => h264_low_latency_input_args(),
        VideoCodec::H265 if passthrough => h264_low_latency_input_args(),
        VideoCodec::H265 => vec!["-fflags".to_string(), "+discardcorrupt".to_string()],
    }
}

fn h264_low_latency_input_args() -> Vec<String> {
    vec![
        "-analyzeduration".to_string(),
        "0".to_string(),
        "-probesize".to_string(),
        "32768".to_string(),
        "-fflags".to_string(),
        "nobuffer".to_string(),
        "-flags".to_string(),
        "low_delay".to_string(),
        "-max_delay".to_string(),
        "0".to_string(),
    ]
}

fn encode_args(rendition: &Rendition, video_codec: VideoCodec) -> Vec<String> {
    match video_codec {
        VideoCodec::H264 => vec![
            "-c:v".to_string(),
            "libx264".to_string(),
            "-preset".to_string(),
            "ultrafast".to_string(),
            "-tune".to_string(),
            "zerolatency".to_string(),
            "-x264-params".to_string(),
            "repeat-headers=1".to_string(),
            "-g".to_string(),
            "6".to_string(),
            "-keyint_min".to_string(),
            "6".to_string(),
            "-sc_threshold".to_string(),
            "0".to_string(),
            "-bf".to_string(),
            "0".to_string(),
            "-pix_fmt".to_string(),
            "yuv420p".to_string(),
            "-b:v".to_string(),
            rendition.bitrate_arg.clone(),
        ],
        VideoCodec::H265 => vec![
            "-c:v".to_string(),
            "libx265".to_string(),
            "-preset".to_string(),
            "ultrafast".to_string(),
            "-tune".to_string(),
            "zerolatency".to_string(),
            "-x265-params".to_string(),
            "repeat-headers=1:keyint=15:min-keyint=15:scenecut=0:bframes=0:open-gop=0".to_string(),
            "-pix_fmt".to_string(),
            "yuv420p".to_string(),
            "-b:v".to_string(),
            rendition.bitrate_arg.clone(),
        ],
    }
}

fn passthrough_bitstream_filter(codec: VideoCodec) -> &'static str {
    match codec {
        VideoCodec::H264 => "h264_mp4toannexb,dump_extra=freq=keyframe,h264_metadata=aud=insert",
        VideoCodec::H265 => "hevc_mp4toannexb,dump_extra=freq=keyframe",
    }
}

fn raw_muxer(codec: VideoCodec) -> &'static str {
    match codec {
        VideoCodec::H264 => "h264",
        VideoCodec::H265 => "hevc",
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct StreamProbe {
    pub video_codec: VideoCodec,
    pub bitrate: Option<u64>,
}

pub async fn probe_rtsp_source(rtsp_url: &str) -> anyhow::Result<StreamProbe> {
    let (url, credentials) = rtsp_url_and_credentials(rtsp_url)?;
    let session = tokio::time::timeout(
        Duration::from_secs(15),
        retina::client::Session::describe(
            url,
            retina::client::SessionOptions::default()
                .creds(credentials)
                .user_agent("cstream-moq-publisher".to_string()),
        ),
    )
    .await
    .context("timed out probing RTSP rendition source")??;

    let stream = session
        .streams()
        .iter()
        .find(|stream| stream.media() == "video")
        .context("RTSP source did not report a video stream")?;
    let video_codec = video_codec_from_rtsp_encoding(stream.encoding_name())
        .context("RTSP video stream is not H.264 or H.265")?;

    Ok(StreamProbe {
        video_codec,
        bitrate: bitrate_from_rtsp_url(rtsp_url),
    })
}

fn video_codec_from_rtsp_encoding(encoding_name: &str) -> Option<VideoCodec> {
    match encoding_name.to_ascii_lowercase().as_str() {
        "h264" => Some(VideoCodec::H264),
        "h265" | "hevc" => Some(VideoCodec::H265),
        _ => None,
    }
}

fn bitrate_from_rtsp_url(raw: &str) -> Option<u64> {
    let url = url::Url::parse(raw).ok()?;
    for (key, value) in url.query_pairs() {
        if matches!(
            key.to_ascii_lowercase().as_str(),
            "videobitrate" | "videomaxbitrate" | "bitrate"
        ) {
            let value: u64 = value.parse().ok()?;
            if value == 0 {
                return None;
            }
            return Some(if value < 100_000 {
                value * 1_000
            } else {
                value
            });
        }
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn late_frame_clock_uses_shared_media_time() {
        let media = MediaClock::new();
        let mut first_track = FrameClock::new(media);
        let first = first_track.next_micros();

        std::thread::sleep(Duration::from_millis(25));

        let mut late_track = FrameClock::new(media);
        let late = late_track.next_micros();

        assert!(
            late >= first + 10_000,
            "late track should start on the shared media timeline"
        );
    }

    #[test]
    fn frame_clock_is_monotonic_within_track() {
        let media = MediaClock::new();
        let mut clock = FrameClock::new(media);

        let first = clock.next_micros();
        let second = clock.next_micros();

        assert!(
            second >= first + FrameClock::MIN_FRAME_MICROS,
            "same-track timestamps should always advance"
        );
    }

    #[test]
    fn h265_seed_waits_for_vps_sps_and_pps() {
        let mut seed = ParameterSetSeed::new(VideoCodec::H265);

        assert!(!seed.observe(&[64, 1]).unwrap());
        assert!(!seed.observe(&[66, 1]).unwrap());
        assert!(seed.observe(&[68, 1]).unwrap());

        assert_eq!(
            &seed.init[..],
            &[
                0, 0, 0, 1, 64, 1, // VPS
                0, 0, 0, 1, 66, 1, // SPS
                0, 0, 0, 1, 68, 1, // PPS
            ]
        );
    }

    #[test]
    fn h265_access_units_use_slice_first_segment_flags() {
        assert!(starts_access_unit(VideoCodec::H265, &[64, 1]).unwrap());
        assert!(starts_access_unit(VideoCodec::H265, &[2, 1, 0x80]).unwrap());
        assert!(!starts_access_unit(VideoCodec::H265, &[2, 1, 0]).unwrap());

        assert!(is_slice_nal(VideoCodec::H265, &[2, 1]).unwrap());
        assert!(!is_slice_nal(VideoCodec::H265, &[64, 1]).unwrap());
    }

    #[test]
    fn rtsp_encoding_names_detect_h264_and_h265() {
        assert_eq!(
            video_codec_from_rtsp_encoding("H264"),
            Some(VideoCodec::H264)
        );
        assert_eq!(
            video_codec_from_rtsp_encoding("h265"),
            Some(VideoCodec::H265)
        );
        assert_eq!(
            video_codec_from_rtsp_encoding("HEVC"),
            Some(VideoCodec::H265)
        );
        assert_eq!(video_codec_from_rtsp_encoding("jpeg"), None);
    }

    #[test]
    fn rtsp_url_credentials_are_moved_to_retina_options() {
        let (url, credentials) =
            rtsp_url_and_credentials("rtsp://user:pass@camera.local/live?camera=1").unwrap();

        assert_eq!(url.as_str(), "rtsp://camera.local/live?camera=1");
        assert_eq!(
            credentials,
            Some(retina::client::Credentials {
                username: "user".to_string(),
                password: "pass".to_string(),
            })
        );
    }

    #[test]
    fn rtsp_url_bitrate_fallback_treats_axis_values_as_kbps() {
        assert_eq!(
            bitrate_from_rtsp_url("rtsp://camera.local/live?videomaxbitrate=6000"),
            Some(6_000_000)
        );
    }
}
