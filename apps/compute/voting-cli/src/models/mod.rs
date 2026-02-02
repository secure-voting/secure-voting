use std::{collections::HashMap, io};

use voting_core::models::{candidate_id::CandidateId, profile::Profile};

pub mod cvr;

pub trait ProfileParser<Ballot> {
    type Error;

    fn parse<R: io::Read>(
        &mut self,
        reader: R,
    ) -> Result<(Profile<Ballot>, HashMap<CandidateId, String>), Self::Error>;
}
