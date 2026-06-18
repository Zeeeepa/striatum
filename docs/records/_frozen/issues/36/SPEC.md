# GH #36 - verdict-less completed review recovery

Source: https://github.com/halbritt/striatum/issues/36

## Summary

Gemini review jobs can publish a review artifact and call `complete` without a
verdict. Accepted-review edges then never fire, and the existing override path
used to require a prior verdict row.

## Acceptance

1. Review-capable jobs are steered to `submit-review` rather than `complete`.
2. The daemon refuses bare `complete` for review jobs with an actionable error.
3. Operators can inject an accepted verdict for a completed review that lacks a
   verdict, with an audit rationale.
