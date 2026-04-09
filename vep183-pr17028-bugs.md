## VEP-183 + PR-17028 Bug/Gaps Log

### Fixed in this branch

1. **SR-IOV DRA host device resolution used `vmi.status` instead of metadata files**
   - **Before:** `pkg/virt-launcher/virtwrap/device/hostdevice/sriov/hostdev.go` resolved PCI addresses from `vmi.Status.DeviceStatus.HostDeviceStatuses`.
   - **After:** PCI is resolved via claim/request lookup from downward-API metadata files using `pkg/dra.GetPCIAddressForClaim()`.
   - **Impact:** Aligns SR-IOV DRA with the metadata-file source-of-truth model (including claim template path handling).

2. **JSON stream decoding in DRA metadata reader used `json.Decoder.More()`**
   - **Before:** `pkg/dra/utils.go` used `dec.More()` for top-level JSON stream decoding.
   - **After:** Decoding now loops on `Decode()` until `io.EOF`.
   - **Impact:** Correctly handles KEP-5304 style concatenated top-level JSON objects.

3. **Status-based DRA network reconciliation/controller leftovers**
   - **Before:** Rebase brought back stale status-based pieces from old commits.
   - **After:** Dropped stale virt-handler DRA network commit and kept `pkg/virt-controller/watch/dra/*` deleted during rebase; retained only relevant controller render/template wiring.
   - **Impact:** Removes old status-reconciliation path that conflicts with metadata-file architecture.

### Still open / intentionally not fixed in this branch

1. **No new end-to-end execution in this change**
   - Unit-level updates were added, but no full SR-IOV DRA e2e run was executed in this iteration.

