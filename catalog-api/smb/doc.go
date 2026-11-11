// Package smb provides the SMB share-path helpers (SplitShare, ResolveSub) that confine
// storage roots to a share sub-directory for the Catalogizer API.
//
// The legacy SmbClient and SmbConnectionPool were removed (SMB-FU-1/2/15): they were reachable
// only from handlers that no route served, and those were deleted with them. Live SMB access
// goes through the services and filesystem packages.
package smb
