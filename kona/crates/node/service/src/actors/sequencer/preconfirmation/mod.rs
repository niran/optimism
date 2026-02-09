//! Preconfirmation tracking for the sequencer.
//!
//! When leadership transfers mid-block in a flashblocks-enabled sequencer setup,
//! the new leader needs knowledge of transactions already published in flashblocks
//! by the previous leader. This module subscribes to upstream flashblocks, tracks
//! the ordered transaction sequence, and injects it into payload attributes when
//! the new leader builds its first block.

mod config;
pub use config::PreconfirmationConfig;

pub(crate) mod tracker;
pub(crate) use tracker::PreconfirmationTracker;

#[cfg(feature = "preconfirmations")]
pub(crate) mod subscriber;
#[cfg(feature = "preconfirmations")]
pub(crate) use subscriber::FlashblocksSubscriber;
