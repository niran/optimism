#[cfg(test)]
use crate::{
    SequencerActorError,
    actors::{
        MockOriginSelector, MockSequencerEngineClient,
        sequencer::{PreconfirmationTracker, tests::test_util::test_actor},
    },
};
use alloy_primitives::{Bytes, B256};
use alloy_rpc_types_engine::PayloadAttributes;
use kona_derive::{BuilderError, PipelineErrorKind, test_utils::TestAttributesBuilder};
use kona_protocol::{BlockInfo, L2BlockInfo};
use op_alloy_rpc_types_engine::OpPayloadAttributes;
use rstest::rstest;
use std::{sync::Arc, time::Duration};

#[rstest]
#[case::temp(PipelineErrorKind::Temporary(BuilderError::Custom("".into()).into()), false)]
#[case::reset(PipelineErrorKind::Reset(BuilderError::Custom("".into()).into()), false)]
#[case::critical(PipelineErrorKind::Critical(BuilderError::Custom("".into()).into()), true)]
#[tokio::test]
async fn test_build_unsealed_payload_prepare_payload_attributes_error(
    #[case] forced_error: PipelineErrorKind,
    #[case] expect_err: bool,
) {
    let mut client = MockSequencerEngineClient::new();

    let unsafe_head = L2BlockInfo::default();
    client.expect_get_unsafe_head().times(1).return_once(move || Ok(unsafe_head));
    // Must not be called on critical error
    client.expect_start_build_block().times(0);
    if let PipelineErrorKind::Reset(_) = &forced_error {
        client.expect_reset_engine_forkchoice().times(1).return_once(move || Ok(()));
    }

    let l1_origin = BlockInfo::default();
    let mut origin_selector = MockOriginSelector::new();
    origin_selector.expect_next_l1_origin().times(1).return_once(move |_, _| Ok(l1_origin));

    let attributes_builder = TestAttributesBuilder { attributes: vec![Err(forced_error)] };

    let mut actor = test_actor();
    actor.origin_selector = origin_selector;
    actor.engine_client = client;
    actor.attributes_builder = attributes_builder;

    let result = actor.build_unsealed_payload().await;
    if expect_err {
        assert!(result.is_err());
        assert!(matches!(
            result.unwrap_err(),
            SequencerActorError::AttributesBuilder(PipelineErrorKind::Critical(_))
        ));
    } else {
        assert!(result.is_ok());
    }
}

/// Helper to create an `OpPayloadAttributes` with the given deposit transactions and timestamp.
fn make_attributes(deposit_txs: Vec<Bytes>, timestamp: u64) -> OpPayloadAttributes {
    OpPayloadAttributes {
        payload_attributes: PayloadAttributes {
            timestamp,
            prev_randao: B256::ZERO,
            suggested_fee_recipient: Default::default(),
            parent_beacon_block_root: None,
            withdrawals: None,
        },
        transactions: Some(deposit_txs),
        no_tx_pool: None,
        gas_limit: Some(30_000_000),
        eip_1559_params: None,
        min_base_fee: None,
    }
}

/// Simulates a leadership-transfer catch-up scenario: the new leader is behind the clock and
/// must build multiple blocks. Preconfirmed transactions from the previous leader's flashblocks
/// should be injected only into the first block (matching parent hash), not into subsequent
/// catch-up blocks.
#[tokio::test]
async fn test_preconfirmations_injected_only_into_correct_block_during_catchup() {
    let parent_hash = B256::with_last_byte(0xAA);
    let catchup_parent_hash = B256::with_last_byte(0xBB);

    // The unsafe head for the first block — matches the parent hash the previous leader was
    // building on.
    let unsafe_head_block_n = L2BlockInfo {
        block_info: BlockInfo::new(parent_hash, 100, B256::ZERO, 1000),
        l1_origin: Default::default(),
        seq_num: 0,
    };

    // The unsafe head for the catch-up block — different parent hash after sealing block N+1.
    let unsafe_head_block_n1 = L2BlockInfo {
        block_info: BlockInfo::new(catchup_parent_hash, 101, parent_hash, 1002),
        l1_origin: Default::default(),
        seq_num: 1,
    };

    // L1 origin with a timestamp far enough ahead to avoid sequencer drift issues.
    let l1_origin = BlockInfo::new(B256::ZERO, 50, B256::ZERO, 2000);

    // Set up preconfirmed transactions keyed to parent_hash.
    let tracker = Arc::new(PreconfirmationTracker::new(Duration::from_secs(60)));
    let preconfirmed_txs =
        vec![Bytes::from_static(b"preconf_tx1"), Bytes::from_static(b"preconf_tx2")];
    let payload_id = alloy_rpc_types_engine::PayloadId::new([1; 8]);
    tracker.start_sequence(parent_hash, payload_id, &preconfirmed_txs).await;

    let deposit_tx = Bytes::from_static(b"deposit");

    // TestAttributesBuilder pops from the back, so push in reverse order.
    let attributes_builder = TestAttributesBuilder {
        attributes: vec![
            Ok(make_attributes(vec![deposit_tx.clone()], 1004)), // second call (catch-up)
            Ok(make_attributes(vec![deposit_tx.clone()], 1002)), // first call
        ],
    };

    let mut actor = test_actor();
    actor.preconfirmation_tracker = Some(tracker);
    actor.attributes_builder = attributes_builder;

    // First block (N+1): builds on parent_hash — should get preconfirmed txs.
    let result = actor.build_attributes(unsafe_head_block_n, l1_origin).await.unwrap().unwrap();
    let txs = result.attributes().transactions.as_ref().unwrap();
    assert_eq!(txs.len(), 3, "Expected deposit + 2 preconfirmed txs");
    assert_eq!(txs[0], deposit_tx, "First tx should be deposit");
    assert_eq!(txs[1], Bytes::from_static(b"preconf_tx1"));
    assert_eq!(txs[2], Bytes::from_static(b"preconf_tx2"));

    // Catch-up block (N+2): different parent hash — should NOT get preconfirmed txs.
    let result = actor.build_attributes(unsafe_head_block_n1, l1_origin).await.unwrap().unwrap();
    let txs = result.attributes().transactions.as_ref().unwrap();
    assert_eq!(txs.len(), 1, "Catch-up block should only have deposit tx");
    assert_eq!(txs[0], deposit_tx);
}

/// Verifies that preconfirmed transactions are injected even when `should_use_tx_pool` returns
/// false (e.g., sequencer drift exceeded). The forced transactions list is always processed by
/// the EL regardless of `no_tx_pool`.
#[tokio::test]
async fn test_preconfirmations_injected_when_no_tx_pool() {
    let parent_hash = B256::with_last_byte(0xCC);

    let unsafe_head = L2BlockInfo {
        block_info: BlockInfo::new(parent_hash, 100, B256::ZERO, 1000),
        l1_origin: Default::default(),
        seq_num: 0,
    };

    // L1 origin with an old timestamp so that the block exceeds sequencer drift,
    // causing should_use_tx_pool to return false.
    let l1_origin = BlockInfo::new(B256::ZERO, 50, B256::ZERO, 100);

    let tracker = Arc::new(PreconfirmationTracker::new(Duration::from_secs(60)));
    let preconfirmed = vec![Bytes::from_static(b"preconf")];
    let payload_id = alloy_rpc_types_engine::PayloadId::new([2; 8]);
    tracker.start_sequence(parent_hash, payload_id, &preconfirmed).await;

    let deposit_tx = Bytes::from_static(b"deposit");
    let attributes_builder = TestAttributesBuilder {
        attributes: vec![Ok(make_attributes(vec![deposit_tx.clone()], 1002))],
    };

    let mut actor = test_actor();
    actor.preconfirmation_tracker = Some(tracker);
    actor.attributes_builder = attributes_builder;

    let result = actor.build_attributes(unsafe_head, l1_origin).await.unwrap().unwrap();

    // no_tx_pool should be true (sequencer drift exceeded).
    assert_eq!(result.attributes().no_tx_pool, Some(true));

    // Preconfirmed txs should still be present in the forced transaction list.
    let txs = result.attributes().transactions.as_ref().unwrap();
    assert_eq!(txs.len(), 2);
    assert_eq!(txs[0], deposit_tx);
    assert_eq!(txs[1], Bytes::from_static(b"preconf"));
}
