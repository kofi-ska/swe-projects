use std::collections::HashSet;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SourceTruthState {
    Valid,
    Missing,
    Conflict,
}

pub fn evaluate(source_hashes: &[String]) -> SourceTruthState {
    if source_hashes.is_empty() {
        return SourceTruthState::Missing;
    }

    if source_hashes.iter().any(|hash| hash.trim().is_empty()) {
        return SourceTruthState::Conflict;
    }

    let mut unique = HashSet::with_capacity(source_hashes.len());
    for hash in source_hashes {
        unique.insert(hash.as_str());
    }
    if unique.len() != source_hashes.len() {
        return SourceTruthState::Conflict;
    }

    SourceTruthState::Valid
}
