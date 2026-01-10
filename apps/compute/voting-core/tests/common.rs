use std::sync::Once;

use ctor::ctor;
use tracing_subscriber::{EnvFilter, fmt};
use voting_core::profile::Profile;

pub const MEMPHIS: usize = 0;
pub const NASHVILLE: usize = 1;
pub const CHATTANOOGA: usize = 2;
pub const KNOXVILLE: usize = 3;

pub fn construct_tennessee_wiki_example() -> Profile {
    let mut votes = Vec::with_capacity(100);

    (0..42).for_each(|_| votes.push(vec![MEMPHIS, NASHVILLE, CHATTANOOGA, KNOXVILLE]));
    (0..26).for_each(|_| votes.push(vec![NASHVILLE, CHATTANOOGA, KNOXVILLE, MEMPHIS]));
    (0..15).for_each(|_| votes.push(vec![CHATTANOOGA, KNOXVILLE, NASHVILLE, MEMPHIS]));
    (0..17).for_each(|_| votes.push(vec![KNOXVILLE, CHATTANOOGA, NASHVILLE, MEMPHIS]));

    Profile::try_from(votes).unwrap()
}

static INIT: Once = Once::new();

#[ctor]
pub fn init_tracing() {
    INIT.call_once(|| {
        let subscriber = fmt()
            .with_env_filter(EnvFilter::from_default_env())
            .with_test_writer()
            .with_line_number(true)
            .with_file(true)
            .finish();
        let _ = tracing::subscriber::set_global_default(subscriber);
    });
}
