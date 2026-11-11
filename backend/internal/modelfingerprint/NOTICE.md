Numerical fingerprint algorithm ported from lmfpd-sdk 0.2.0, MIT.
Upstream: Ikaleio/lm-detector, commit 1e58ed83c1cde17f88cba89560ad4d083042ae92.
The supplied references/magpie-fingerprint/vendor/lmfpd-sdk distribution was
used as the implementation reference. See LICENSE for upstream attribution.

Ranking is a closed-set comparison within the downloaded reference bank. It
cannot establish model identity. Reference data is fetched separately, only
on an explicit administrator request. The detector bytes are checked against
the manifest SHA-256; this binds parts to a manifest, not an independently
trusted publisher signature.
