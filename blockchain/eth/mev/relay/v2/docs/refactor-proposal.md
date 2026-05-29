# v2 Refactor Proposal

## Why

The previous relay kept too much lifecycle, retry, persistence, and health logic in one place.

The refactor splits that into:

- validation
- admission scoring
- identity helpers
- terminal settlement
- persistence helpers
- worker and retry processing
- typed error mapping

## Goals

- reduce bug surface
- make the lifecycle readable
- make operational signals explicit
- make alerts possible from stable metrics
- make the code easier to hand off
- keep behavior bounded and conservative

## Non-goals

- no rewrite
- no microservice split
- no new broker or state engine
- no topology change

## Acceptance criteria

- compile cleanly across the module
- preserve current behavior for valid and invalid submissions
- preserve bounded queue and retry semantics
- keep ready/unsafe decisions deterministic
- expose alert-ready metrics
