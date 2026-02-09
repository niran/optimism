//! Background task that subscribes to upstream flashblocks and populates the tracker.

use super::tracker::PreconfirmationTracker;
use alloy_primitives::B256;
use alloy_rpc_types_engine::PayloadId;
use futures::StreamExt;
use op_alloy_rpc_types_engine::OpFlashblockPayload;
use std::{collections::HashMap, sync::Arc, time::Duration};
use tokio_tungstenite::{connect_async, tungstenite::Message};
use tokio_util::sync::CancellationToken;
use url::Url;

/// Interval between TTL eviction sweeps.
const EVICTION_INTERVAL: Duration = Duration::from_secs(10);

/// Delay before reconnecting after a WebSocket disconnection.
const RECONNECT_DELAY: Duration = Duration::from_secs(1);

/// Background task that subscribes to an upstream flashblocks WebSocket and
/// populates a [`PreconfirmationTracker`] with the ordered transaction sequence.
///
/// Maintains a `PayloadId → parent_hash` mapping so that follow-up flashblocks
/// (index > 0) can be associated with the correct sequence — only the first
/// flashblock in a sequence carries the `base` with `parent_hash`.
pub(crate) struct FlashblocksSubscriber {
    ws_url: Url,
    tracker: Arc<PreconfirmationTracker>,
    cancellation: CancellationToken,
}

impl FlashblocksSubscriber {
    /// Creates a new [`FlashblocksSubscriber`].
    pub(crate) const fn new(
        ws_url: Url,
        tracker: Arc<PreconfirmationTracker>,
        cancellation: CancellationToken,
    ) -> Self {
        Self { ws_url, tracker, cancellation }
    }

    /// Runs the subscriber loop with automatic reconnection.
    ///
    /// This method runs until the cancellation token is triggered. On WebSocket
    /// disconnection, it waits briefly and reconnects.
    pub(crate) async fn run(self) {
        let mut eviction_interval = tokio::time::interval(EVICTION_INTERVAL);
        // Maps PayloadId to parent_hash for resolving follow-up flashblocks.
        let mut payload_parents: HashMap<PayloadId, B256> = HashMap::new();

        loop {
            if self.cancellation.is_cancelled() {
                return;
            }

            info!(target: "sequencer::preconfirmation", url = %self.ws_url, "Connecting to flashblocks WebSocket");

            let ws_stream = tokio::select! {
                _ = self.cancellation.cancelled() => return,
                result = connect_async(self.ws_url.as_str()) => {
                    match result {
                        Ok((stream, _)) => {
                            info!(target: "sequencer::preconfirmation", "Connected to flashblocks WebSocket");
                            stream
                        }
                        Err(err) => {
                            warn!(
                                target: "sequencer::preconfirmation",
                                ?err,
                                "Failed to connect to flashblocks WebSocket, retrying"
                            );
                            tokio::time::sleep(RECONNECT_DELAY).await;
                            continue;
                        }
                    }
                }
            };

            let (_, mut read) = ws_stream.split();

            loop {
                tokio::select! {
                    _ = self.cancellation.cancelled() => return,
                    _ = eviction_interval.tick() => {
                        self.tracker.evict_expired().await;
                    }
                    msg = read.next() => {
                        match msg {
                            Some(Ok(Message::Text(text))) => {
                                self.handle_message(text.as_bytes(), &mut payload_parents).await;
                            }
                            Some(Ok(Message::Binary(data))) => {
                                self.handle_message(&data, &mut payload_parents).await;
                            }
                            Some(Ok(Message::Ping(_) | Message::Pong(_) | Message::Frame(_))) => {
                                // Tungstenite handles ping/pong automatically.
                            }
                            Some(Ok(Message::Close(_))) => {
                                info!(target: "sequencer::preconfirmation", "WebSocket closed by server, reconnecting");
                                break;
                            }
                            Some(Err(err)) => {
                                warn!(target: "sequencer::preconfirmation", ?err, "WebSocket error, reconnecting");
                                break;
                            }
                            None => {
                                info!(target: "sequencer::preconfirmation", "WebSocket stream ended, reconnecting");
                                break;
                            }
                        }
                    }
                }
            }

            // Clear payload-to-parent mapping on disconnect since sequences are
            // no longer valid.
            payload_parents.clear();
            tokio::time::sleep(RECONNECT_DELAY).await;
        }
    }

    /// Parses a flashblock message and updates the tracker.
    async fn handle_message(&self, data: &[u8], payload_parents: &mut HashMap<PayloadId, B256>) {
        let flashblock: OpFlashblockPayload = match serde_json::from_slice(data) {
            Ok(fb) => fb,
            Err(err) => {
                warn!(
                    target: "sequencer::preconfirmation",
                    ?err,
                    "Failed to deserialize flashblock payload"
                );
                return;
            }
        };

        if flashblock.index == 0 {
            if let Some(ref base) = flashblock.base {
                let parent_hash = base.parent_hash;
                payload_parents.insert(flashblock.payload_id, parent_hash);

                self.tracker
                    .start_sequence(
                        parent_hash,
                        flashblock.payload_id,
                        &flashblock.diff.transactions,
                    )
                    .await;

                debug!(
                    target: "sequencer::preconfirmation",
                    parent = %parent_hash,
                    payload_id = ?flashblock.payload_id,
                    tx_count = flashblock.diff.transactions.len(),
                    "Started new preconfirmation sequence"
                );
            } else {
                warn!(
                    target: "sequencer::preconfirmation",
                    payload_id = ?flashblock.payload_id,
                    "Flashblock index 0 missing base payload"
                );
            }
        } else if let Some(&parent_hash) = payload_parents.get(&flashblock.payload_id) {
            self.tracker
                .append_transactions(
                    parent_hash,
                    flashblock.payload_id,
                    &flashblock.diff.transactions,
                )
                .await;

            debug!(
                target: "sequencer::preconfirmation",
                parent = %parent_hash,
                index = flashblock.index,
                tx_count = flashblock.diff.transactions.len(),
                "Appended transactions to preconfirmation sequence"
            );
        } else {
            debug!(
                target: "sequencer::preconfirmation",
                payload_id = ?flashblock.payload_id,
                index = flashblock.index,
                "Ignoring flashblock with unknown payload_id"
            );
        }
    }
}
