use std::{
    collections::{BTreeMap, BTreeSet},
    path::{Path, PathBuf},
    sync::{Arc, Mutex},
    time::Duration,
};

use anyhow::Context;
use hang::catalog::VideoConfig;
use moq_mux::catalog::hang::Catalog;
use serde::Deserialize;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Rendition {
    pub name: String,
    pub width: u32,
    pub height: u32,
    pub bitrate_arg: String,
    pub bitrate: Option<u64>,
    pub passthrough: bool,
}

impl std::str::FromStr for Rendition {
    type Err = anyhow::Error;

    fn from_str(raw: &str) -> Result<Self, Self::Err> {
        let raw = raw.trim();
        if raw.eq_ignore_ascii_case("passthrough") {
            return Ok(Self {
                name: "passthrough".to_string(),
                width: 0,
                height: 0,
                bitrate_arg: String::new(),
                bitrate: None,
                passthrough: true,
            });
        }

        let parts: Vec<_> = raw.split(':').collect();
        if parts.len() == 2 && parts[1].trim().eq_ignore_ascii_case("passthrough") {
            let name = validate_rendition_name(parts[0])?;
            return Ok(Self {
                name,
                width: 0,
                height: 0,
                bitrate_arg: String::new(),
                bitrate: None,
                passthrough: true,
            });
        }

        anyhow::ensure!(
            parts.len() == 3,
            "rendition must use name:WIDTHxHEIGHT:BITRATE, passthrough, or name:passthrough"
        );

        let name = validate_rendition_name(parts[0])?;

        let dimensions: Vec<_> = parts[1].split('x').collect();
        anyhow::ensure!(
            dimensions.len() == 2,
            "rendition dimensions must use WIDTHxHEIGHT"
        );
        let width: u32 = dimensions[0]
            .parse()
            .context("rendition width must be a positive integer")?;
        let height: u32 = dimensions[1]
            .parse()
            .context("rendition height must be a positive integer")?;
        anyhow::ensure!(
            width > 0 && height > 0,
            "rendition dimensions must be positive"
        );

        Ok(Self {
            name: name.to_string(),
            width,
            height,
            bitrate_arg: parts[2].trim().to_string(),
            bitrate: parse_bitrate(parts[2].trim())?,
            passthrough: false,
        })
    }
}

fn validate_rendition_name(raw: &str) -> anyhow::Result<String> {
    let name = raw.trim();
    anyhow::ensure!(!name.is_empty(), "rendition name is required");
    anyhow::ensure!(
        !name.contains(char::is_whitespace) && !name.contains(':') && !name.contains('/'),
        "rendition name must not contain whitespace, ':' or '/'"
    );
    Ok(name.to_string())
}

fn parse_bitrate(raw: &str) -> anyhow::Result<Option<u64>> {
    if raw.is_empty() {
        return Ok(None);
    }

    let (number, multiplier) = match raw.as_bytes().last().copied() {
        Some(b'k') | Some(b'K') => (&raw[..raw.len() - 1], 1_000),
        Some(b'm') | Some(b'M') => (&raw[..raw.len() - 1], 1_000_000),
        _ => (raw, 1),
    };

    let value: u64 = number
        .parse()
        .context("rendition bitrate must be a positive integer with optional k/m suffix")?;
    anyhow::ensure!(value > 0, "rendition bitrate must be positive");
    Ok(Some(value * multiplier))
}

#[derive(Clone)]
pub struct FilteredCatalog {
    producer: moq_mux::catalog::Producer,
    state: Arc<Mutex<State>>,
}

#[derive(Default)]
struct State {
    renditions: Vec<Rendition>,
    labels: BTreeMap<String, String>,
    active: Option<BTreeSet<String>>,
    full: Catalog,
    last_published: Option<Catalog>,
}

impl FilteredCatalog {
    pub fn new(
        broadcast: &mut hang::moq_net::BroadcastProducer,
        renditions: Vec<Rendition>,
        active: Vec<String>,
    ) -> Result<Self, hang::moq_net::Error> {
        let producer = moq_mux::catalog::Producer::new(broadcast)?;
        let active = if active.is_empty() {
            None
        } else {
            Some(active.into_iter().collect())
        };

        Ok(Self {
            producer,
            state: Arc::new(Mutex::new(State {
                renditions,
                labels: BTreeMap::new(),
                active,
                full: Catalog::default(),
                last_published: None,
            })),
        })
    }

    pub fn sync_from(&self, catalog: Catalog) -> anyhow::Result<()> {
        let mut state = self.state.lock().expect("catalog lock poisoned");
        let labels = state.video_labels(&catalog.video.renditions);
        state.full = catalog;
        state.labels = labels;
        drop(state);
        self.publish_if_changed()
    }

    pub fn upsert_video_catalog(&self, label: &str, catalog: Catalog) -> anyhow::Result<()> {
        let mut state = self.state.lock().expect("catalog lock poisoned");
        let next_tracks: BTreeSet<_> = catalog.video.renditions.keys().cloned().collect();
        let old_tracks: Vec<_> = state
            .labels
            .iter()
            .filter_map(|(track_name, track_label)| {
                (track_label == label && !next_tracks.contains(track_name))
                    .then(|| track_name.clone())
            })
            .collect();

        for track_name in old_tracks {
            state.labels.remove(&track_name);
            state.full.video.renditions.remove(&track_name);
        }

        for (track_name, mut config) in catalog.video.renditions {
            if let Some(rendition) = state
                .renditions
                .iter()
                .find(|rendition| rendition.name == label)
                && let Some(bitrate) = rendition.bitrate
            {
                config.bitrate = Some(bitrate);
            }
            state.labels.insert(track_name.clone(), label.to_string());
            state.full.video.renditions.insert(track_name, config);
        }

        drop(state);
        self.publish_if_changed()
    }

    pub fn remove_label(&self, label: &str) -> anyhow::Result<()> {
        let mut state = self.state.lock().expect("catalog lock poisoned");
        let old_tracks: Vec<_> = state
            .labels
            .iter()
            .filter_map(|(track_name, track_label)| {
                (track_label == label).then(|| track_name.clone())
            })
            .collect();

        for track_name in old_tracks {
            state.labels.remove(&track_name);
            state.full.video.renditions.remove(&track_name);
        }

        drop(state);
        self.publish_if_changed()
    }

    pub fn set_active(&self, active: Option<BTreeSet<String>>) -> anyhow::Result<()> {
        let mut state = self.state.lock().expect("catalog lock poisoned");
        state.active = active;
        drop(state);
        self.publish_if_changed()
    }

    pub fn is_active(&self, label: &str) -> bool {
        let state = self.state.lock().expect("catalog lock poisoned");
        state
            .active
            .as_ref()
            .is_none_or(|active| active.contains(label))
    }

    fn publish_if_changed(&self) -> anyhow::Result<()> {
        let next = {
            let state = self.state.lock().expect("catalog lock poisoned");
            let filtered = state.filtered_catalog();
            if state.last_published.as_ref() == Some(&filtered) {
                return Ok(());
            }
            filtered
        };

        let mut producer = self.producer.clone();
        let mut catalog = producer.lock();
        *catalog = next.clone();
        drop(catalog);

        let mut state = self.state.lock().expect("catalog lock poisoned");
        state.last_published = Some(next);
        Ok(())
    }
}

impl State {
    fn filtered_catalog(&self) -> Catalog {
        let mut filtered = self.full.clone();

        filtered.video.renditions.clear();

        for (track_name, config) in &self.full.video.renditions {
            let label = self.labels.get(track_name).unwrap_or(track_name);
            if self
                .active
                .as_ref()
                .is_none_or(|active| active.contains(label))
            {
                let mut config = config.clone();
                if let Some(rendition) = self
                    .renditions
                    .iter()
                    .find(|rendition| rendition.name == label.as_str())
                    && let Some(bitrate) = rendition.bitrate
                {
                    config.bitrate = Some(bitrate);
                }
                filtered.video.renditions.insert(track_name.clone(), config);
            }
        }

        filtered
    }

    fn video_labels(&self, renditions: &BTreeMap<String, VideoConfig>) -> BTreeMap<String, String> {
        renditions
            .keys()
            .enumerate()
            .map(|(index, track_name)| {
                let label = self
                    .renditions
                    .get(index)
                    .map(|rendition| rendition.name.clone())
                    .unwrap_or_else(|| track_name.clone());
                (track_name.clone(), label)
            })
            .collect()
    }
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ControlFile {
    advertise: Option<Vec<String>>,
    renditions: Option<Vec<String>>,
    video: Option<Vec<String>>,
}

impl ControlFile {
    fn active(self) -> Option<BTreeSet<String>> {
        self.advertise
            .or(self.renditions)
            .or(self.video)
            .map(|names| names.into_iter().collect())
    }
}

pub async fn read_control_file(path: impl AsRef<Path>) -> anyhow::Result<Option<BTreeSet<String>>> {
    let path = path.as_ref();
    let raw = tokio::fs::read_to_string(path)
        .await
        .with_context(|| format!("read catalog control file {}", path.display()))?;
    parse_control_file(path, &raw)
}

pub async fn watch_control_file(
    catalog: FilteredCatalog,
    path: impl Into<PathBuf>,
) -> anyhow::Result<()> {
    let path = path.into();
    let mut last_applied = None;
    let mut last_invalid = None;

    loop {
        match tokio::fs::read_to_string(&path).await {
            Ok(raw)
                if last_applied.as_ref() == Some(&raw) || last_invalid.as_ref() == Some(&raw) => {}
            Ok(raw) => match parse_control_file(&path, &raw) {
                Ok(active) => {
                    catalog.set_active(active)?;
                    last_applied = Some(raw);
                    last_invalid = None;
                }
                Err(err) => {
                    tracing::warn!(?err, "ignoring invalid catalog control file");
                    last_invalid = Some(raw);
                }
            },
            Err(err) if err.kind() == std::io::ErrorKind::NotFound => {}
            Err(err) => {
                return Err(err)
                    .with_context(|| format!("read catalog control file {}", path.display()));
            }
        }

        tokio::time::sleep(Duration::from_millis(250)).await;
    }
}

fn parse_control_file(path: &Path, raw: &str) -> anyhow::Result<Option<BTreeSet<String>>> {
    let control: ControlFile =
        serde_json::from_str(raw).with_context(|| control_file_error(path))?;
    Ok(control.active())
}

fn control_file_error(path: &Path) -> String {
    format!(
        "read catalog control file {}; expected {{\"advertise\":[\"360p\"]}} or {{\"advertise\":[\"360p\",\"720p\"]}}",
        path.display()
    )
}

#[cfg(test)]
mod tests {
    use std::time::{SystemTime, UNIX_EPOCH};

    use super::*;

    #[test]
    fn parses_encoded_rendition() {
        let got: Rendition = "720p:1280x720:2500k"
            .parse()
            .expect("parse encoded rendition");

        assert_eq!(
            got,
            Rendition {
                name: "720p".to_string(),
                width: 1280,
                height: 720,
                bitrate_arg: "2500k".to_string(),
                bitrate: Some(2_500_000),
                passthrough: false,
            }
        );
    }

    #[test]
    fn parses_passthrough_rendition() {
        let got: Rendition = "passthrough".parse().expect("parse passthrough");

        assert_eq!(
            got,
            Rendition {
                name: "passthrough".to_string(),
                width: 0,
                height: 0,
                bitrate_arg: String::new(),
                bitrate: None,
                passthrough: true,
            }
        );
    }

    #[test]
    fn parses_named_passthrough_rendition() {
        let got: Rendition = "original:passthrough"
            .parse()
            .expect("parse named passthrough");

        assert_eq!(
            got,
            Rendition {
                name: "original".to_string(),
                width: 0,
                height: 0,
                bitrate_arg: String::new(),
                bitrate: None,
                passthrough: true,
            }
        );
    }

    #[tokio::test]
    async fn watcher_ignores_invalid_json_and_applies_later_update() {
        let path = std::env::temp_dir().join(format!(
            "cstream-catalog-control-{}.json",
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .expect("system clock before epoch")
                .as_nanos()
        ));
        tokio::fs::write(&path, r#"{"advertise":["360p"]}"#)
            .await
            .expect("write initial control file");

        let mut broadcast = hang::moq_net::Broadcast::new().produce();
        let catalog = FilteredCatalog::new(
            &mut broadcast,
            vec![
                "360p:640x360:800k".parse().expect("parse 360p rendition"),
                "720p:1280x720:2500k".parse().expect("parse 720p rendition"),
            ],
            vec!["360p".to_string()],
        )
        .expect("create catalog");

        let watcher = tokio::spawn(watch_control_file(catalog.clone(), path.clone()));

        wait_for(|| catalog.is_active("360p") && !catalog.is_active("720p")).await;

        tokio::fs::write(&path, "{\n  \"advertise\": [\n    \"360p\",\n    \"")
            .await
            .expect("write partial control file");
        tokio::time::sleep(Duration::from_millis(400)).await;
        assert!(catalog.is_active("360p"));
        assert!(!catalog.is_active("720p"));
        assert!(!watcher.is_finished(), "watcher should keep running");

        tokio::fs::write(&path, r#"{"advertise":["360p","720p"]}"#)
            .await
            .expect("write valid control file");
        wait_for(|| catalog.is_active("360p") && catalog.is_active("720p")).await;

        watcher.abort();
        let _ = tokio::fs::remove_file(path).await;
    }

    async fn wait_for(mut predicate: impl FnMut() -> bool) {
        let deadline = tokio::time::Instant::now() + Duration::from_secs(2);
        loop {
            if predicate() {
                return;
            }
            assert!(
                tokio::time::Instant::now() < deadline,
                "timed out waiting for predicate"
            );
            tokio::time::sleep(Duration::from_millis(20)).await;
        }
    }
}
