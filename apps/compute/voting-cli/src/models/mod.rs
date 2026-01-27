use std::{collections::HashMap, io};

use voting_core::profile::{CandidateId, Profile};

pub mod cvr;

pub trait ProfileParser {
    type Error;

    fn parse<R: io::Read>(
        &mut self,
        reader: R,
    ) -> Result<(Profile, HashMap<CandidateId, String>), Self::Error>;
}
