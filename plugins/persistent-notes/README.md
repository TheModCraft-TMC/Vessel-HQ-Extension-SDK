# Persistent notes write-back extension

This native page demonstrates real, persistent writes from an extension to Vessel.
It saves a shared UTF-8 note (up to 16 KiB) in the extension's registration, with
independent `storage.read` and `storage.write` grants. It does not change Docker,
Kubernetes or other infrastructure. Do not put secrets in these shared notes.

```sh
node scripts/package-plugin.mjs --vessel-root /path/to/Vessel-HQ --plugin persistent-notes --out /tmp/persistent-notes.vesselhq
```

Upload the archive at **Settings → Extensions**, review the package, select the
native UI, note-read and note-write permissions, and acknowledge native UI trust.
Compile it using the [development operator controller](https://github.com/TheModCraft-TMC/Vessel-HQ/blob/feature/native-extensions-prerelease/scripts/native-extensions/README.md).
Open `/extensions/persistent-notes` in that console, edit the note and save it.
Reload the page to read the value back from the backend datastore.

The host binds every storage call to the registration ID, package digest and
approval revision. Each read/write checks fresh native UI trust and the relevant
storage grant. Only administrators can use this first storage API. A write includes
the last read storage version: two concurrent editors receive a conflict instead
of silently overwriting each other. Reloading explicitly discards local edits.
Disabling the extension blocks subsequent storage operations. Reupload preserves
the saved note but clears all grants, requiring a fresh approval. Removing the
registration removes its note. No filesystem paths or arbitrary object keys are
accepted. Successful reads and writes are audited; note contents stay out of the
audit payload. Writes and their audit entries commit in the same transaction.

The Go SDK can read and update the same note using independent
`storage.service.read` and `storage.service.write` machine grants. These do not
inherit browser UI trust or user sessions. See the
[SDK guide](../../docs/go-sdk.md) and its persistent-notes CLI. A private
shared Unix socket supports a container with no network and no published ports;
mTLS still binds its identity to the approved package. Both clients use the same
version checks and transactional audit. This does not grant infrastructure writes.
The UI is trusted native code and shares the browser's console context, as with
all native extensions. Source checks are compatibility checks, not a sandbox.
