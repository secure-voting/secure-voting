use std::{collections::HashMap, io};

use thiserror::Error;
use voting_core::models::{
    candidate_id::CandidateId,
    profile::Profile,
    ranking::{ProfileError, RankingBallot},
};

use crate::models::ProfileParser;

pub struct CVRParser;

#[derive(Error, Debug)]
pub enum CVRError {
    /// Missing required header
    #[error("A required header is missing: {0}")]
    MissingHeader(String),
    /// CSV Error
    #[error(transparent)]
    CSVError(#[from] csv::Error),
    /// Profile creation error
    #[error(transparent)]
    ProfileError(#[from] ProfileError),
    /// Unsupported feature
    #[error("Unsupported feature: {0}")]
    Unsupported(String),
    /// Invalid rank
    #[error("Rank is invalid, should be a number")]
    InvalidRank,
}

const REQUIRED_HEADERS: [&str; 3] = ["ballot_id", "rank", "choice"];
const BALLOT_ID: usize = 0;
const RANK: usize = 1;
const CHOICE: usize = 2;

impl ProfileParser<RankingBallot> for CVRParser {
    type Error = CVRError;

    fn parse<R: io::Read>(
        &mut self,
        reader: R,
    ) -> Result<(Profile<RankingBallot>, HashMap<CandidateId, String>), Self::Error> {
        let mut rdr = csv::Reader::from_reader(reader);
        let headers = rdr.headers()?;

        let mut header_mapping = vec![None; REQUIRED_HEADERS.len()];

        for (id, header) in headers.iter().enumerate() {
            if let Some(idx) = REQUIRED_HEADERS.iter().position(|x| x == &header) {
                header_mapping[idx] = Some(id);
            }
        }

        if let Some(missing) = header_mapping.iter().position(Option::is_none) {
            return Err(CVRError::MissingHeader(
                REQUIRED_HEADERS[missing].to_owned(),
            ));
        }

        let header_mapping = header_mapping
            .iter()
            .copied()
            .map(Option::unwrap)
            .collect::<Vec<_>>();

        let mut candidate_mapping: HashMap<String, usize> = HashMap::new();
        let mut reverse_map: HashMap<CandidateId, String> = HashMap::new();
        let mut vote_map: HashMap<String, Vec<usize>> = HashMap::new();
        let mut new_cand_id = 0;

        for record in rdr.records() {
            match record {
                Ok(record) => {
                    let ballot_id = &record[header_mapping[BALLOT_ID]];
                    let rank = &record[header_mapping[RANK]];
                    let choice = record[header_mapping[CHOICE]].to_owned();

                    let Ok(rank) = rank.parse::<usize>() else {
                        return Err(CVRError::InvalidRank);
                    };

                    let rank = rank - 1;

                    if choice == "$WRITE_IN" || choice == "$UNDERVOTE" {
                        return Err(CVRError::Unsupported(choice));
                    }

                    if !candidate_mapping.contains_key(&choice) {
                        candidate_mapping.insert(choice.clone(), new_cand_id);
                        reverse_map.insert(
                            CandidateId::new(new_cand_id, choice.clone()),
                            choice.clone(),
                        );
                        new_cand_id += 1;
                    }

                    let vec = vote_map.entry(ballot_id.to_owned()).or_default();
                    while vec.len() <= rank {
                        vec.push(0);
                    }

                    vec[rank] = candidate_mapping[&choice];
                }
                Err(e) => return Err(e.into()),
            }
        }

        let votes = vote_map.into_values().collect::<Vec<_>>();

        Ok((
            Profile::try_from((votes, candidate_mapping.keys().cloned().collect()))?,
            reverse_map,
        ))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn crv_example_data_unsupported() {
        let data = "ballot_id,rank,choice\nAB001,1,Alice\nAB001,2,Bob\nAB001,3,Edna\nAB002,1,$WRITE_IN\nAB002,2,Edna\nAB002,3,$UNDERVOTE";

        assert!(matches!(
            CVRParser.parse(data.as_bytes()),
            Err(CVRError::Unsupported(_))
        ));
    }
    #[test]
    fn crv_example_data_fixed_to_support() {
        let data = "ballot_id,rank,choice\nAB001,1,Alice\nAB001,2,Bob\nAB001,3,Edna\nAB002,1,Alice\nAB002,2,Edna\nAB002,3,Bob";

        assert!(CVRParser.parse(data.as_bytes()).is_ok());
    }
}
