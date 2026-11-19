# EnvDoctor M4 dependency review — 2026-11-19

The Go 1.26.11 module graph was read with `go list -m -json all`. Exact versions and
checksums are committed in `go.mod`/`go.sum`; `go mod verify` passed and
`govulncheck ./...` reported **No vulnerabilities found** on the final M4 implementation.

Bubble Tea v1.3.10 and `golang.org/x/term` v0.48.0 follow accepted ADR 0006.
`golang.org/x/sys` v0.50.0, `github.com/charmbracelet/x/ansi` v0.10.1 and
`golang.org/x/text` v0.44.0 are also pinned. The binary still builds with CGO disabled.

The reviewed graph uses MIT, BSD-3-Clause and the existing Apache-2.0 dependencies.
`github.com/mattn/go-localereader` v0.0.1 has no standalone LICENSE file in its
downloaded module. Its pinned README explicitly declares **MIT** and names Yasuhiro
Matsumoto as the author; that README, rather than a nonexistent LICENSE, is the
evidence recorded below. These are source declarations reviewed for this dependency
set, not a claim that all upstream modules ship identical license files.

The table records SHA-256 of the exact source file read from the Go module cache.
Pseudo-versions identify their exact revisions. The existing schema test dependency
and other modules in the selected graph are included even when absent from the final
binary dependency list.

| Module | Version | Declared license | Evidence file | SHA-256 |
|---|---|---|---|---|
| github.com/aymanbagabas/go-osc52/v2 | v2.0.1 | MIT | LICENSE | 11319e34d9dac2f45392f14f4f62e28b24173cfa13d39b32e7b7b66c2f3bdccd |
| github.com/charmbracelet/bubbletea | v1.3.10 | MIT | LICENSE | 69793670485e4ae8bf28fcbe5639141f604256d5bb6c1862571f3a32823fa221 |
| github.com/charmbracelet/colorprofile | v0.2.3-0.20250311203215-f60798e515dc | MIT | LICENSE | 57d6eeeb336030b89c0e0d12cf869f84b97a481cd00b350ef21fca995da5a39a |
| github.com/charmbracelet/lipgloss | v1.1.0 | MIT | LICENSE | 699cffa88b62691ea09706d356e5ee0982e87ca2b1c58ca682f27d6ae87f9efc |
| github.com/charmbracelet/x/ansi | v0.10.1 | MIT | LICENSE | 0baa4a5942a801d93f4050d2bc0039d8c037105fcd335230d118eec99a11782b |
| github.com/charmbracelet/x/cellbuf | v0.0.13-0.20250311204145-2c3ea96c31dd | MIT | LICENSE | 0baa4a5942a801d93f4050d2bc0039d8c037105fcd335230d118eec99a11782b |
| github.com/charmbracelet/x/term | v0.2.1 | MIT | LICENSE | 0baa4a5942a801d93f4050d2bc0039d8c037105fcd335230d118eec99a11782b |
| github.com/erikgeiser/coninput | v0.0.0-20211004153227-1c3628e74d0f | MIT | LICENSE | 0a00deeeb36bb7b8ccbb9003100b665e584d9384e66c8c26f29b5929ecaa745b |
| github.com/inconshreveable/mousetrap | v1.1.0 | Apache-2.0 | LICENSE | 4819582701f150b28a563a6cd8ed0bf5a754e3c67b90ad38d78ba4131bf77795 |
| github.com/lucasb-eyer/go-colorful | v1.2.0 | MIT | LICENSE | 1e4cf2a117516ec1b54041d68ed4f5d331878b04de3386f4790e7b5b9c04e1ec |
| github.com/mattn/go-isatty | v0.0.20 | MIT | LICENSE | 08eab1118c80885fa1fa6a6dd7303f65a379fcb3733e063d20d1bbc2c76e6fa1 |
| github.com/mattn/go-localereader | v0.0.1 | MIT (README declaration) | README.md | 0c5c52517f13becd7a1e1234f1e8a5ba370f5a5078cee9ac058c2d534e267fcb |
| github.com/mattn/go-runewidth | v0.0.16 | MIT | LICENSE | 88a2379b3ca34bf5c57127aff9dcb802bbb60ece0805cdbda65b3bd115f971d9 |
| github.com/muesli/ansi | v0.0.0-20230316100256-276c6243b2f6 | MIT | LICENSE | 43f65e00e8919fa43c5f45bb2ae578b6d3ee823eb99aa14c85c9ecd06e0350b5 |
| github.com/muesli/cancelreader | v0.2.2 | MIT | LICENSE | 8c139dbbc83b4fb055b4100b6b9402f7de3dd6bbca05015bdfcc4b392c8cfd5f |
| github.com/muesli/termenv | v0.16.0 | MIT | LICENSE | 233e8cf9a812999ccee4d3edb270b0f11ab3539658e680bcb873604915ca5038 |
| github.com/rivo/uniseg | v0.4.7 | MIT | LICENSE.txt | a59885f5f0f3b3c07cf9444db5fe399b6f0791eac82055f7b85cc65500551039 |
| github.com/santhosh-tekuri/jsonschema/v5 | v5.3.1 | Apache-2.0 | LICENSE | c8858a5a76440bbca484e134cf7df46385d090dd18b2c58e650f939258802e5b |
| github.com/spf13/cobra | v1.10.2 | Apache-2.0 | LICENSE.txt | 5e3400b93bbb099e83e52bab885e7441750673c21f97988ca3f1240639b63283 |
| github.com/spf13/pflag | v1.0.9 | BSD-3-Clause | LICENSE | b8514c577c1c4b46cee454d5a882b15fa411e72c5bd7f801f241591789fce61a |
| github.com/xo/terminfo | v0.0.0-20220910002029-abceb7e1c41e | MIT | LICENSE | 1f5f51151ba96f0492ce288bd007242f924fdad9f04002e175e951644340a891 |
| golang.org/x/exp | v0.0.0-20220909182711-5c715a9e8561 | BSD-3-Clause | LICENSE | 2d36597f7117c38b006835ae7f537487207d8ec407aa9d9980794b2030cbc067 |
| golang.org/x/sys | v0.50.0 | BSD-3-Clause | LICENSE | 911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad |
| golang.org/x/term | v0.48.0 | BSD-3-Clause | LICENSE | 911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad |
| golang.org/x/text | v0.44.0 | BSD-3-Clause | LICENSE | 911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad |

The README screenshot displays the implemented Windows PTY output through a
temporary xterm.js 6.0.0 preview. That QA dependency and the preview server live in
ignored `bin/`; neither is a runtime dependency, shipped feature or CI requirement.

Native Windows tests/race and the bounded renderer lifecycle tests cover terminal
integration. Native Windows/Ubuntu/macOS GitHub CI passed tests, race and builds;
compilation, model tests and executed CI are recorded separately in [M4 validation](m4-validation.md).
