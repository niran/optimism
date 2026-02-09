//! Configuration for preconfirmation tracking.

use std::time::Duration;
use url::Url;

/// Default TTL for preconfirmation entries.
const DEFAULT_PRECONFIRMATION_TTL_SECS: u64 = 60;

/// Configuration for preconfirmation tracking.
///
/// When enabled, the sequencer subscribes to upstream flashblocks via WebSocket
/// and tracks the ordered transaction sequence. On leadership transfer, the new
/// leader injects previously preconfirmed transactions into its first block's
/// payload attributes.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PreconfirmationConfig {
    /// Whether preconfirmation tracking is enabled.
    pub enabled: bool,
    /// WebSocket URL for upstream flashblocks subscription.
    pub flashblocks_url: Option<Url>,
    /// How long to retain tracked sequences before eviction.
    pub ttl: Duration,
}

impl Default for PreconfirmationConfig {
    fn default() -> Self {
        Self {
            enabled: false,
            flashblocks_url: None,
            ttl: Duration::from_secs(DEFAULT_PRECONFIRMATION_TTL_SECS),
        }
    }
}
