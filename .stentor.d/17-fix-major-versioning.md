Fixed a bug where `gotagger` calculated the wrong major version for go modules.

When finding the latest tag for a go module,
`gotagger` was only considering tags that matched the major version of the module.
This caused `gotagger` to essentially always calculate the version as `v1.0.0`.
The filtering was changed to only filter out tags whose major version are greater than the module version.
