use std::io;

use voting_core::profile::Profile;

mod cvr;

pub trait ProfileParser {
    type Error;

    fn parse<R: io::Read>(&mut self, reader: R) -> Result<Profile, Self::Error>;
}
