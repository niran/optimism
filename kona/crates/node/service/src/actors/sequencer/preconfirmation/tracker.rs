//! In-memory store for preconfirmed transaction orderings.

use alloy_primitives::{Bytes, B256};
use alloy_rpc_types_engine::PayloadId;
use std::{
    collections::HashMap,
    time::{Duration, Instant},
};
use tokio::sync::RwLock;

/// An entry tracking the ordered transaction list for a single block sequence.
#[derive(Debug)]
struct PreconfirmationEntry {
    /// The ordered transaction list, built by appending each flashblock's diff.transactions.
    ordered_transactions: Vec<Bytes>,
    /// PayloadId of the sequence (from the first flashblock).
    payload_id: PayloadId,
    /// When the first flashblock in this sequence was received.
    received_at: Instant,
}

/// Thread-safe in-memory store of ordered transaction lists per parent hash.
///
/// Each flashblock sequence is identified by its parent hash. As differential
/// flashblocks arrive, their transactions are appended in order. When the
/// sequencer reads the tracked sequence via [`take_transactions`], the entry
/// is drained (consume-once semantics) to prevent re-injection.
///
/// [`take_transactions`]: PreconfirmationTracker::take_transactions
#[derive(Debug)]
pub(crate) struct PreconfirmationTracker {
    /// Ordered transactions by parent block hash.
    entries: RwLock<HashMap<B256, PreconfirmationEntry>>,
    /// How long entries are retained before eviction.
    ttl: Duration,
}

impl PreconfirmationTracker {
    /// Creates a new [`PreconfirmationTracker`] with the given TTL.
    pub(crate) fn new(ttl: Duration) -> Self {
        Self { entries: RwLock::new(HashMap::new()), ttl }
    }

    /// Starts a new sequence for the given parent hash.
    ///
    /// Called when flashblock index 0 arrives. Creates a new entry, replacing
    /// any existing one for this parent hash.
    pub(crate) async fn start_sequence(
        &self,
        parent_hash: B256,
        payload_id: PayloadId,
        initial_txs: &[Bytes],
    ) {
        let mut entries = self.entries.write().await;
        entries.insert(
            parent_hash,
            PreconfirmationEntry {
                ordered_transactions: initial_txs.to_vec(),
                payload_id,
                received_at: Instant::now(),
            },
        );
    }

    /// Appends transactions from a subsequent flashblock to the existing entry.
    ///
    /// Only appends if an entry exists for this parent hash and its payload_id
    /// matches. This prevents mixing sequences from different builders.
    pub(crate) async fn append_transactions(
        &self,
        parent_hash: B256,
        payload_id: PayloadId,
        txs: &[Bytes],
    ) {
        let mut entries = self.entries.write().await;
        if let Some(entry) = entries.get_mut(&parent_hash) {
            if entry.payload_id == payload_id {
                entry.ordered_transactions.extend_from_slice(txs);
            }
        }
    }

    /// Takes the ordered transaction list for the given parent hash, removing
    /// the entry (consume-once semantics).
    ///
    /// Returns an empty vec if no entry exists or the entry has expired.
    pub(crate) async fn take_transactions(&self, parent_hash: B256) -> Vec<Bytes> {
        let mut entries = self.entries.write().await;
        match entries.remove(&parent_hash) {
            Some(entry) if entry.received_at.elapsed() < self.ttl => entry.ordered_transactions,
            _ => Vec::new(),
        }
    }

    /// Removes entries older than the configured TTL.
    pub(crate) async fn evict_expired(&self) {
        let mut entries = self.entries.write().await;
        let ttl = self.ttl;
        entries.retain(|_, entry| entry.received_at.elapsed() < ttl);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Duration;

    fn test_payload_id(val: u8) -> PayloadId {
        PayloadId::new([val; 8])
    }

    #[tokio::test]
    async fn test_start_and_take_sequence() {
        let tracker = PreconfirmationTracker::new(Duration::from_secs(60));
        let parent = B256::with_last_byte(1);
        let pid = test_payload_id(1);
        let txs = vec![Bytes::from_static(b"tx1"), Bytes::from_static(b"tx2")];

        tracker.start_sequence(parent, pid, &txs).await;

        let result = tracker.take_transactions(parent).await;
        assert_eq!(result.len(), 2);
        assert_eq!(result[0], Bytes::from_static(b"tx1"));
        assert_eq!(result[1], Bytes::from_static(b"tx2"));
    }

    #[tokio::test]
    async fn test_consume_once_semantics() {
        let tracker = PreconfirmationTracker::new(Duration::from_secs(60));
        let parent = B256::with_last_byte(1);
        let pid = test_payload_id(1);
        let txs = vec![Bytes::from_static(b"tx1")];

        tracker.start_sequence(parent, pid, &txs).await;

        // First take returns transactions.
        let result = tracker.take_transactions(parent).await;
        assert_eq!(result.len(), 1);

        // Second take returns empty.
        let result = tracker.take_transactions(parent).await;
        assert!(result.is_empty());
    }

    #[tokio::test]
    async fn test_append_transactions() {
        let tracker = PreconfirmationTracker::new(Duration::from_secs(60));
        let parent = B256::with_last_byte(1);
        let pid = test_payload_id(1);

        tracker.start_sequence(parent, pid, &[Bytes::from_static(b"tx1")]).await;
        tracker
            .append_transactions(
                parent,
                pid,
                &[Bytes::from_static(b"tx2"), Bytes::from_static(b"tx3")],
            )
            .await;

        let result = tracker.take_transactions(parent).await;
        assert_eq!(result.len(), 3);
        assert_eq!(result[0], Bytes::from_static(b"tx1"));
        assert_eq!(result[1], Bytes::from_static(b"tx2"));
        assert_eq!(result[2], Bytes::from_static(b"tx3"));
    }

    #[tokio::test]
    async fn test_payload_id_mismatch_rejected() {
        let tracker = PreconfirmationTracker::new(Duration::from_secs(60));
        let parent = B256::with_last_byte(1);
        let pid1 = test_payload_id(1);
        let pid2 = test_payload_id(2);

        tracker.start_sequence(parent, pid1, &[Bytes::from_static(b"tx1")]).await;
        // Append with a different payload_id should be ignored.
        tracker.append_transactions(parent, pid2, &[Bytes::from_static(b"tx2")]).await;

        let result = tracker.take_transactions(parent).await;
        assert_eq!(result.len(), 1);
        assert_eq!(result[0], Bytes::from_static(b"tx1"));
    }

    #[tokio::test]
    async fn test_ttl_expiry() {
        let tracker = PreconfirmationTracker::new(Duration::from_millis(10));
        let parent = B256::with_last_byte(1);
        let pid = test_payload_id(1);

        tracker.start_sequence(parent, pid, &[Bytes::from_static(b"tx1")]).await;

        // Wait for TTL to expire.
        tokio::time::sleep(Duration::from_millis(20)).await;

        let result = tracker.take_transactions(parent).await;
        assert!(result.is_empty());
    }

    #[tokio::test]
    async fn test_evict_expired() {
        let tracker = PreconfirmationTracker::new(Duration::from_millis(10));
        let parent1 = B256::with_last_byte(1);
        let parent2 = B256::with_last_byte(2);
        let pid = test_payload_id(1);

        tracker.start_sequence(parent1, pid, &[Bytes::from_static(b"tx1")]).await;

        // Wait for TTL to expire.
        tokio::time::sleep(Duration::from_millis(20)).await;

        // Add a fresh entry.
        tracker.start_sequence(parent2, pid, &[Bytes::from_static(b"tx2")]).await;

        tracker.evict_expired().await;

        // Expired entry should be gone.
        let result1 = tracker.take_transactions(parent1).await;
        assert!(result1.is_empty());

        // Fresh entry should remain.
        let result2 = tracker.take_transactions(parent2).await;
        assert_eq!(result2.len(), 1);
    }

    #[tokio::test]
    async fn test_start_sequence_replaces_existing() {
        let tracker = PreconfirmationTracker::new(Duration::from_secs(60));
        let parent = B256::with_last_byte(1);
        let pid1 = test_payload_id(1);
        let pid2 = test_payload_id(2);

        tracker.start_sequence(parent, pid1, &[Bytes::from_static(b"old")]).await;
        tracker.start_sequence(parent, pid2, &[Bytes::from_static(b"new")]).await;

        let result = tracker.take_transactions(parent).await;
        assert_eq!(result.len(), 1);
        assert_eq!(result[0], Bytes::from_static(b"new"));
    }

    #[tokio::test]
    async fn test_take_nonexistent_parent_returns_empty() {
        let tracker = PreconfirmationTracker::new(Duration::from_secs(60));
        let parent = B256::with_last_byte(99);

        let result = tracker.take_transactions(parent).await;
        assert!(result.is_empty());
    }
}
