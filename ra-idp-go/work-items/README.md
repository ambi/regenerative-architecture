# Retrieving Work Item Context

While the Work Item YAML files are the canonical records, AI agents should initially read only the following fields:

1. `motivation`
2. `scope`
3. `out_of_scope`
4. `initial_context`
5. `affected_guarantees`
6. `verification`
7. `risk`

Access large `completion` fields or validation evidence only when historical audits or past verification results are required.
For `initial_context`, prioritize directory-based `features` and feature directories rather than long lists of individual files. List specific file paths only when they represent exceptional entry points located outside the feature directories.
