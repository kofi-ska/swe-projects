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

    let mut unique = HashSet::with_capacity(source_hashes.len());
    for hash in source_hashes {
        let normalized = hash.trim();
        if normalized.is_empty() {
            return SourceTruthState::Conflict;
        }
        unique.insert(normalized);
    }

    if unique.len() != source_hashes.len() {
        return SourceTruthState::Conflict;
    }

    SourceTruthState::Valid
}
