# Translations

## TLDR
Single canonical source file generates both Android and iOS localization files, keeping the two mobile platforms from drifting apart on copy.

## Capabilities
- One source of truth generates Android `strings.xml` and iOS `Localizable.strings`
- Higher-confidence languages, usable as-is: Afrikaans, isiZulu, isiXhosa, Sesotho, Setswana
- Lower-confidence languages, need native speaker review before real use: Sepedi, Xitsonga, siSwati, Tshivenda, isiNdebele

## Implementation
- Canonical translation source file (generator not yet built per [[Roadmap]] step 7)

## Status
Content drafted for all SA official languages; generation tooling and native-speaker review still open — see [[Roadmap]].

## Related
- [[Roadmap]]
