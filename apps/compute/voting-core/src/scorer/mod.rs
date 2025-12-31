use crate::types::Profile;

pub trait Scorer {
    fn score_ballot(ballot: &[usize], scores: &mut [usize]);
    fn compute_score(profile: &Profile) -> Vec<usize> {
        let n_candidates = profile.n_candidates();
        let n_voters = profile.n_voters();

        let mut scores = vec![0; n_candidates];

        for i in 0..n_voters {
            Self::score_ballot(&profile[i], &mut scores);
        }

        scores
    }
}
