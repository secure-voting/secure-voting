use std::ops::Index;

pub type CandidateId = usize;

#[derive(Debug, Clone)]
pub struct Profile {
    votes: Vec<Vec<usize>>,
}

impl Profile {
    pub fn n_candidates(&self) -> usize {
        self.votes[0].len()
    }

    pub fn n_voters(&self) -> usize {
        self.votes.len()
    }
}

impl Index<usize> for Profile {
    type Output = Vec<usize>;

    fn index(&self, index: usize) -> &Self::Output {
        &self.votes[index]
    }
}

impl TryFrom<Vec<Vec<usize>>> for Profile {
    type Error = &'static str;

    fn try_from(value: Vec<Vec<usize>>) -> Result<Self, Self::Error> {
        match (1..value.len()).any(|row| value[row].len() != value[0].len()) {
            true => Err("Some votes are different length"),
            false => Ok(Profile { votes: value }),
        }
    }
}
