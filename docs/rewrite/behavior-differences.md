# Intentional differences

- Go backend uses a versioned SQLite document repository for legacy DTOs, keeping optional fields intact; no Spring/Flyway/Quartz database runtime.
- bcrypt passwords, persisted random JWT secret and revocable sessions replace MD5/default-secret behavior. MD5 is accepted only for imported users and upgraded on successful login.
- Full rebuild never clears arbitrary user files. Only owned, unmodified obsolete outputs are quarantined.
- Output directory is user-configurable; Linux container-only prefixes removed from UI.
- Telemetry is disabled. Third-party API credentials are not included in build/release artifacts.
- Cron uses the Go Quartz parser (including L/W/#) rather than robfig's limited Quartz subset. Five-field conversion currently retains upstream controller behavior.
- Remote write retries are conservative: ambiguous uploads/moves stop for reconciliation, rather than blindly repeating.
- dev removed autoRenameMedia compared with main. Go keeps the optional backend field for the accepted design while preserving dev's manual workflows.

Still validating: manual flat-movie/season organization details, AI identification fallbacks, exact log cursor compatibility, and original parser edge cases. Native compatibility is tracked separately from cross-compilation.
