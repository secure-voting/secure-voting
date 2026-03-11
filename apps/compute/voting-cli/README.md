[![unsafe forbidden](https://img.shields.io/badge/unsafe-forbidden-success.svg)](https://github.com/rust-secure-code/safety-dance/)
[![MSRV](https://img.shields.io/badge/MSRV-1.92.0-blue)](https://github.com/schukark/secure-voting)
![CI](https://github.com/schukark/secure-voting/actions/workflows/rust-ci.yml/badge.svg)
![licenses](https://img.shields.io/badge/licenses-MIT%2FApache--2.0-blue)

# Voting CLI

`voting-cli` is a helper crate that uses the `voting-core` crate as its backbone and provides a user-friendly CLI frontend.

The CLI utility allows to choose any rule that is implemented in the standard set of rules and evaluate it on the supplied file.

## Supported rules

1. Plurality
2. Approval with q=2,3
3. InversePlurality
4. Borda
5. Black
6. Copeland I/II/III
7. Simpson (aka Maxmin)
8. Minmax
9. Hare
10. Nanson
11. Coombs
12. InverseBorda

## Supported file types

1. [RCV](https://github.com/ranked-vote/rcv-data-format?tab=readme-ov-file)
