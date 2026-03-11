use std::{collections::HashMap, io};

use voting_core::models::{candidate_id::CandidateId, profile::Profile};

pub mod rcv;

type ParseResult<Ballot, Error> = Result<(Profile<Ballot>, HashMap<CandidateId, String>), Error>;

pub trait ProfileParser<Ballot> {
    type Error;

    fn parse<R: io::Read>(&mut self, reader: R) -> ParseResult<Ballot, Self::Error>;
}
