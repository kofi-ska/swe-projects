pub const MAX_QUOTE_AGE: u32 = 8;

pub fn is_fresh(quote_age: u32) -> bool {
    quote_age <= MAX_QUOTE_AGE
}
