# Go rewrite progress

Implementation branch: dev. Starting SHA: 9f3ae5cee9bcb4c8bf09f2b8fb1495340f8f4ae7.
Design baseline: e25c766753c758f00aa47818057d3d9df0b4969a.

## Implemented / under verification

- Go 1.27.1, embedded Nuxt UI, SQLite records/settings/sessions, CLI and native service adapter.
- All 62 legacy routes registered, plus task run status/cancellation.
- OpenList CRUD, recursive listing, QPS/QPM, safe STRM writes, successful manifests and owned output quarantine.
- TMDB/AI recognition, metadata download/NFO generation, manual preview/rename/upload jobs, Emby/Jellyfin and Apprise.
- Legacy import into a new database, consistent SQLite backup, platform default data paths.
- Six native CI targets and portable/install package generation. Release uses dev -> beta -> main.

## Executed checks

- `go test -race ./...`: passed initial functional suite on macOS ARM64.
- Cross compilation with CGO_ENABLED=0: darwin/linux/windows x amd64/arm64 passed.
- Built macOS ARM64 binary: embedded UI, authentication, STRM generation, restart persistence smoke passed.
- Java 21 baseline tests: UrlEncoderTest, TaskManifestServiceTest, TaskDirectoryStructureValidatorTest, SeasonDirectoryNameParserTest passed.
- Nuxt static generation passed; inherited typecheck issues being corrected.

## Remaining validation

Native CI, installer lifecycle, browser E2E, expanded manual-job failure fixtures and full API contract comparison are still running/being added. No Go release has been published. Do not equate route registration with verified behavioral parity.
