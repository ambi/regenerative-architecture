# UI Features

The `features/` directory represents the UI context boundaries aligned with the features defined in RA/SCL.
Unless they are cross-cutting common components, views, local components, and local helpers must reside under their corresponding feature directory.

Do not use the alias name `slices/`. This avoids confusion with Go's standard `slices` package and makes it easier for AI agents to determine the scope of files they need to read.
