//go:build nasleg

// Package nasclients is the READ-ONLY harness of the real-NAS client leg (scripts/test-infra/nas_clients_leg.sh, docs/scripts/nas_clients_leg.md).
// It drives the NEW protocol clients of submodules/filesystem (pkg/ftp FTP and explicit FTPS with certificate pinning, pkg/sftp with host-key pinning, pkg/nfs3)
// against the owner's Synology hosts through the fabric chain Confined > Retrying > Limited > ReadOnly > tripwire > client. It does not modify any package.
//
// Safety properties, each of them recorded as evidence and none of them an assumption:
//   - the ONLY write-class path to the client is a TRIPWIRE below decorators.ReadOnly: a write-class call that reached it is refused there (nothing is ever sent to a NAS) and counted;
//   - one HostBudget per host (1 concurrent, 1 start/second); the observed minimum gap between the client-level requests of a host is measured at the wire-facing layer;
//   - bounded sample: depth <= 3, <= 2000 entries, <= 80 listing requests per share; ONE file per share read, at most 1 MiB, the bytes are discarded after a sha256;
//   - names are never recorded (counts, sha256 of the sorted top-level names, extension histograms); credentials and addresses are scrubbed from every recorded string.
//
// Environment (a 0600 env file handed to the container by name, never argv): NCL_USER, NCL_PW, NCL_IP_<n>; non-secret: NCL_PROTO (ftps|ftp|sftp|nfs), NCL_HOSTS (2,3,..),
// NCL_REF (directory with the reference evidence: ftps-<n>.json, sftp-<n>.json, survey-<n>.json), NCL_OUT (result directory).
package nasclients

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"digital.vasic.filesystem/pkg/client"
	"digital.vasic.filesystem/pkg/decorators"
	"digital.vasic.filesystem/pkg/fabric"
	"digital.vasic.filesystem/pkg/ftp"
	"digital.vasic.filesystem/pkg/nfs3"
	"digital.vasic.filesystem/pkg/sftp"
)

const (
	maxDepth        = 3
	maxEntries      = 2000
	maxListPerShare = 80
	maxReadBytes    = 1 << 20
	chainName       = "Confined>Retrying>Limited>ReadOnly>tripwire>client"
	confirmedBy     = "SIMULATED-OWNER-CONFIRMATION: fingerprint recorded by the wp12 protocol survey (nas-protocols), not confirmed out of band by the owner"
)

var (
	reIPv4  = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`)
	errTrip = errors.New("tripwire: a write-class call reached the layer below ReadOnly and was refused")
)

func scrub(s string) string {
	for _, v := range []string{os.Getenv("NCL_PW"), os.Getenv("NCL_USER")} {
		if len(v) >= 3 {
			s = strings.ReplaceAll(s, v, "<redacted>")
		}
	}
	for i := 1; i <= 7; i++ {
		if ip := os.Getenv("NCL_IP_" + strconv.Itoa(i)); ip != "" {
			s = strings.ReplaceAll(s, ip, "<host>")
		}
	}
	s = reIPv4.ReplaceAllString(s, "<ip>")
	if len(s) > 240 {
		s = s[:240] + "..."
	}
	return s
}

type errInfo struct {
	Code    int    `json:"ftp_reply_code,omitempty"`
	Depth   int    `json:"depth,omitempty"`
	Type    string `json:"type"`
	Class   string `json:"fabric_class"`
	Message string `json:"message,omitempty"`
}

var reQuoted = regexp.MustCompile("\"[^\"]*\"|'[^']*'|`[^`]*`")

// redactMsg keeps the shape of an error without any entry name: the listed path and its base name, and every quoted string, are replaced.
func redactMsg(err error, p string) string {
	m := err.Error()
	if p != "" && p != "/" {
		m = strings.ReplaceAll(m, p, "<path>")
		if b := path.Base(p); len(b) >= 2 {
			m = strings.ReplaceAll(m, b, "<name>")
		}
	}
	return scrub(reQuoted.ReplaceAllString(m, "<q>"))
}

func ei(err error, withMsg bool) *errInfo {
	if err == nil {
		return nil
	}
	e := &errInfo{Type: fmt.Sprintf("%T", errors.Unwrap(err)), Class: fabric.Classify(err).String()}
	var te *textproto.Error
	if errors.As(err, &te) {
		e.Code = te.Code
	}
	if errors.Unwrap(err) == nil {
		e.Type = fmt.Sprintf("%T", err)
	}
	if withMsg {
		e.Message = scrub(err.Error())
	}
	return e
}

// ---- spy / tripwire (below ReadOnly) ----

type spy struct {
	client.Client
	mu       sync.Mutex
	calls    map[string]int
	reqStart []time.Time // List/Read/Info/Exists starts
	listDur  []time.Duration
	trips    []string
}

func newSpy(in client.Client) *spy { return &spy{Client: in, calls: map[string]int{}} }
func (s *spy) note(m string, req bool) {
	s.mu.Lock()
	s.calls[m]++
	if req {
		s.reqStart = append(s.reqStart, time.Now())
	}
	s.mu.Unlock()
}
func (s *spy) trip(m string) error {
	s.mu.Lock()
	s.calls[m]++
	s.trips = append(s.trips, m)
	s.mu.Unlock()
	return errTrip
}
func (s *spy) ListDirectory(ctx context.Context, p string) ([]*client.FileInfo, error) {
	s.note("ListDirectory", true)
	t0 := time.Now()
	out, err := s.Client.ListDirectory(ctx, p)
	d := time.Since(t0)
	s.mu.Lock()
	s.listDur = append(s.listDur, d)
	s.mu.Unlock()
	return out, err
}
func (s *spy) ReadFile(ctx context.Context, p string) (io.ReadCloser, error) {
	s.note("ReadFile", true)
	return s.Client.ReadFile(ctx, p)
}
func (s *spy) GetFileInfo(ctx context.Context, p string) (*client.FileInfo, error) {
	s.note("GetFileInfo", true)
	return s.Client.GetFileInfo(ctx, p)
}
func (s *spy) FileExists(ctx context.Context, p string) (bool, error) {
	s.note("FileExists", true)
	return s.Client.FileExists(ctx, p)
}
func (s *spy) WriteFile(context.Context, string, io.Reader) error { return s.trip("WriteFile") }
func (s *spy) DeleteFile(context.Context, string) error           { return s.trip("DeleteFile") }
func (s *spy) CopyFile(context.Context, string, string) error     { return s.trip("CopyFile") }
func (s *spy) CreateDirectory(context.Context, string) error      { return s.trip("CreateDirectory") }
func (s *spy) DeleteDirectory(context.Context, string) error      { return s.trip("DeleteDirectory") }

func (s *spy) minGapSeconds() (float64, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.reqStart)
	if n < 2 {
		return -1, n
	}
	min := time.Hour
	for i := 1; i < n; i++ {
		if g := s.reqStart[i].Sub(s.reqStart[i-1]); g < min {
			min = g
		}
	}
	return round(min.Seconds(), 3), n
}

func round(f float64, d int) float64 {
	p := 1.0
	for i := 0; i < d; i++ {
		p *= 10
	}
	return float64(int64(f*p+0.5)) / p
}

func buildChain(raw client.Client, b *fabric.HostBudget) (client.Client, *spy, error) {
	sp := newSpy(raw)
	ro := decorators.ReadOnly(sp)
	lim := fabric.Limited(ro, b)
	ret, err := fabric.Retrying(lim, fabric.RetryPolicy{MaxAttempts: 3, BaseDelay: 500 * time.Millisecond, MaxDelay: 2 * time.Second})
	if err != nil {
		return nil, nil, err
	}
	conf, err := fabric.Confined(ret, "/")
	if err != nil {
		return nil, nil, err
	}
	return conf, sp, nil
}

// ---- measurements ----

type latency struct {
	N     int     `json:"n"`
	P50   float64 `json:"p50_ms"`
	P95   float64 `json:"p95_ms"`
	Max   float64 `json:"max_ms"`
	Mean  float64 `json:"mean_ms"`
	Valid bool    `json:"-"`
}

func lat(ds []time.Duration) latency {
	if len(ds) == 0 {
		return latency{}
	}
	f := make([]float64, len(ds))
	var sum float64
	for i, d := range ds {
		f[i] = float64(d.Microseconds()) / 1000
		sum += f[i]
	}
	sort.Float64s(f)
	pc := func(p float64) float64 { return round(f[int(float64(len(f)-1)*p+0.5)], 2) }
	return latency{N: len(f), P50: pc(0.5), P95: pc(0.95), Max: round(f[len(f)-1], 2), Mean: round(sum/float64(len(f)), 2)}
}

type extCount struct {
	Ext   string `json:"ext"`
	Count int    `json:"count"`
}

func extOf(name string) string {
	e := strings.ToLower(path.Ext(name))
	if e == "" || e == "." {
		return "(none)"
	}
	e = strings.TrimPrefix(e, ".")
	if len(e) > 12 {
		return "(long)"
	}
	return e
}

func namesSHA(names []string) string {
	sort.Strings(names)
	h := sha256.Sum256([]byte(strings.Join(names, "\n")))
	return hex.EncodeToString(h[:])
}

type readSample struct {
	Status        string   `json:"status"`
	Selection     string   `json:"selection,omitempty"`
	SizeClass     string   `json:"file_size_class,omitempty"`
	BytesRead     int64    `json:"bytes_read"`
	SHA256        string   `json:"sha256_of_bytes_read,omitempty"`
	TTFBms        float64  `json:"ttfb_ms,omitempty"`
	TotalS        float64  `json:"total_s,omitempty"`
	MiBsTotal     float64  `json:"mib_per_s_total,omitempty"`
	MiBsAfterTTFB float64  `json:"mib_per_s_after_first_byte,omitempty"`
	ContentStored bool     `json:"content_stored"`
	Error         *errInfo `json:"error,omitempty"`
}

type shareResult struct {
	Share             string         `json:"share"`
	Status            string         `json:"status"`
	Error             *errInfo       `json:"error,omitempty"`
	TopLevelEntries   int            `json:"top_level_entries"`
	TopLevelDirs      int            `json:"top_level_dirs"`
	TopLevelFiles     int            `json:"top_level_files"`
	TopLevelNamesSHA  string         `json:"top_level_names_sha256"`
	SampledEntries    int            `json:"sampled_entries"`
	SampledFiles      int            `json:"sampled_files"`
	SampledDirs       int            `json:"sampled_dirs"`
	ListingRequests   int            `json:"listing_requests"`
	ListErrors        []errInfo      `json:"list_errors"`
	ListErrorCount    int            `json:"list_error_count"`
	TruncatedByBound  bool           `json:"truncated_by_bound"`
	MaxDepth          int            `json:"max_depth"`
	ExtHist           []extCount     `json:"ext_histogram_top25"`
	Latency           latency        `json:"listing_latency"`
	Read              *readSample    `json:"read_sample,omitempty"`
	SMB               interface{}    `json:"smb_crosscheck"`
	ReadSkippedReason string         `json:"read_skipped_reason,omitempty"`
	ModeHistFiles     map[string]int `json:"mode_histogram_files_top6,omitempty"`
}

type entry struct {
	name string
	size int64
	dir  bool
	mode string
}

// walkShare is the bounded breadth-first sample of one share/export root. Entries are sorted by name so the sample is deterministic across protocols.
func walkShare(ctx context.Context, c client.Client, sp *spy, share, root string, doRead bool) *shareResult {
	r := &shareResult{Share: share, MaxDepth: maxDepth, ListErrors: []errInfo{}}
	type item struct {
		p string
		d int
	}
	q := []item{{root, 0}}
	ext := map[string]int{}
	modes := map[string]int{}
	var cand []struct {
		p    string
		size int64
	}
	listStart := len(sp.listDur)
	reqBefore := sp.calls["ListDirectory"]
	var top []string
	for len(q) > 0 {
		it := q[0]
		q = q[1:]
		if sp.calls["ListDirectory"]-reqBefore >= maxListPerShare || r.SampledEntries >= maxEntries {
			r.TruncatedByBound = true
			break
		}
		lst, err := c.ListDirectory(ctx, it.p)
		if err != nil {
			if it.d == 0 {
				r.Status = "denied_or_error"
				r.Error = ei(err, true)
				r.ListingRequests = sp.calls["ListDirectory"] - reqBefore
				return r
			}
			r.ListErrorCount++
			if len(r.ListErrors) < 5 {
				e := ei(err, false)
				e.Message = redactMsg(err, it.p)
				e.Depth = it.d
				r.ListErrors = append(r.ListErrors, *e)
			}
			continue
		}
		es := make([]entry, 0, len(lst))
		for _, f := range lst {
			if f == nil || f.Name == "" || f.Name == "." || f.Name == ".." {
				continue
			}
			es = append(es, entry{f.Name, f.Size, f.IsDir, fmt.Sprintf("%04o", f.Mode.Perm())})
		}
		sort.Slice(es, func(i, j int) bool { return es[i].name < es[j].name })
		for _, e := range es {
			if it.d == 0 {
				top = append(top, e.name)
				if e.dir {
					r.TopLevelDirs++
				} else {
					r.TopLevelFiles++
				}
			}
			if r.SampledEntries >= maxEntries {
				r.TruncatedByBound = true
				break
			}
			r.SampledEntries++
			if e.dir {
				r.SampledDirs++
				if it.d+1 < maxDepth {
					q = append(q, item{path.Join(it.p, e.name), it.d + 1})
				} else {
					r.TruncatedByBound = true // directories at the depth bound are counted, not listed
				}
			} else {
				r.SampledFiles++
				ext[extOf(e.name)]++
				modes[e.mode]++
				if e.size > 0 {
					cand = append(cand, struct {
						p    string
						size int64
					}{path.Join(it.p, e.name), e.size})
				}
			}
		}
	}
	r.Status = "listed"
	r.TopLevelEntries = len(top)
	r.TopLevelNamesSHA = namesSHA(top)
	r.ListingRequests = sp.calls["ListDirectory"] - reqBefore
	r.Latency = lat(sp.listDur[listStart:])
	hs := make([]extCount, 0, len(ext))
	for k, v := range ext {
		hs = append(hs, extCount{k, v})
	}
	sort.Slice(hs, func(i, j int) bool {
		if hs[i].Count != hs[j].Count {
			return hs[i].Count > hs[j].Count
		}
		return hs[i].Ext < hs[j].Ext
	})
	if len(hs) > 25 {
		hs = hs[:25]
	}
	r.ExtHist = hs
	r.ModeHistFiles = modes
	if !doRead {
		return r
	}
	// selection: the LARGEST file of at most 1 MiB (a whole small file, the most meaningful throughput); if none is that small, the smallest file overall (read capped at 1 MiB). Ties by path.
	sort.Slice(cand, func(i, j int) bool {
		if cand[i].size != cand[j].size {
			return cand[i].size < cand[j].size
		}
		return cand[i].p < cand[j].p
	})
	pick, why := -1, ""
	for i, f := range cand {
		if f.size <= maxReadBytes {
			pick, why = i, "largest_file_of_at_most_1MiB"
		}
	}
	if pick < 0 && len(cand) > 0 {
		pick, why = 0, "smallest_file_overall_capped_1MiB"
	}
	if pick < 0 {
		r.Read = &readSample{Status: "no_candidate_file"}
		return r
	}
	r.Read = readOne(ctx, c, cand[pick].p, cand[pick].size, why)
	return r
}

func readOne(ctx context.Context, c client.Client, p string, size int64, why string) *readSample {
	rs := &readSample{Selection: why}
	switch {
	case size >= maxReadBytes:
		rs.SizeClass = "ge_1MiB"
	case size >= 64<<10:
		rs.SizeClass = "64KiB_to_1MiB"
	default:
		rs.SizeClass = "lt_64KiB"
	}
	t0 := time.Now()
	rc, err := c.ReadFile(ctx, p)
	if err != nil {
		rs.Status, rs.Error = "error", ei(err, false)
		return rs
	}
	defer rc.Close()
	h := sha256.New()
	buf := make([]byte, 64<<10)
	var total int64
	var ttfb time.Duration
	for total < maxReadBytes {
		want := int64(len(buf))
		if rem := maxReadBytes - total; rem < want {
			want = rem
		}
		n, rerr := rc.Read(buf[:want])
		if n > 0 {
			if total == 0 {
				ttfb = time.Since(t0)
			}
			h.Write(buf[:n])
			total += int64(n)
		}
		if rerr != nil {
			if rerr != io.EOF {
				rs.Error = ei(rerr, false)
			}
			break
		}
	}
	el := time.Since(t0)
	rs.BytesRead = total
	rs.SHA256 = hex.EncodeToString(h.Sum(nil))
	rs.TotalS = round(el.Seconds(), 3)
	rs.TTFBms = round(float64(ttfb.Microseconds())/1000, 1)
	if el > 0 {
		rs.MiBsTotal = round(float64(total)/(1<<20)/el.Seconds(), 2)
	}
	if d := el - ttfb; d > 0 && total > 0 {
		rs.MiBsAfterTTFB = round(float64(total)/(1<<20)/d.Seconds(), 2)
	}
	rs.Status = "ok"
	if rs.Error != nil {
		rs.Status = "error_after_partial_read"
	}
	return rs
}

// ---- reference evidence ----

func loadJSON(p string) map[string]interface{} {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return m
}

func smbShares(ref string, n int) map[string]map[string]interface{} {
	out := map[string]map[string]interface{}{}
	m := loadJSON(fmt.Sprintf("%s/survey-%d.json", ref, n))
	if m == nil {
		return out
	}
	arr, _ := m["shares"].([]interface{})
	for _, a := range arr {
		if s, ok := a.(map[string]interface{}); ok {
			if nm, _ := s["share"].(string); nm != "" {
				out[nm] = s
			}
		}
	}
	return out
}

func num(m map[string]interface{}, k string) int {
	f, _ := m[k].(float64)
	return int(f)
}

func smbCross(smb map[string]map[string]interface{}, r *shareResult) interface{} {
	s, ok := smb[r.Share]
	if !ok {
		return map[string]interface{}{"smb_status": "share_absent_in_smb_survey"}
	}
	st, _ := s["status"].(string)
	if st != "listed" {
		out := map[string]interface{}{"smb_status": st, "smb_nt_status": s["nt_status"]}
		if r.Status == "listed" {
			out["finding"] = "SECURITY: share is DENIED over SMB for this account but LISTABLE over this protocol"
		} else {
			out["finding"] = "none: denied over both"
		}
		return out
	}
	out := map[string]interface{}{"smb_status": "listed", "smb_top_level_entries": num(s, "top_level_entries"), "this_top_level_entries": r.TopLevelEntries,
		"top_level_entries_match": num(s, "top_level_entries") == r.TopLevelEntries}
	trunc, _ := s["truncated_by_bound"].(bool)
	out["smb_sampled_entries"], out["this_sampled_entries"] = num(s, "sampled_entries"), r.SampledEntries
	if !trunc && !r.TruncatedByBound {
		out["sampled_entries_comparable"] = true
		out["sampled_entries_match"] = num(s, "sampled_entries") == r.SampledEntries
	} else {
		out["sampled_entries_comparable"] = false
		out["note"] = "at least one side hit its sample bound: totals are not comparable"
	}
	return out
}

// ---- host-level result ----

type hostResult struct {
	Schema          string                   `json:"schema"`
	Alias           string                   `json:"alias"`
	Protocol        string                   `json:"protocol"`
	ClientPackage   string                   `json:"client_package"`
	Chain           string                   `json:"chain"`
	Status          string                   `json:"status"` // works | refused | blocked | error
	Reason          string                   `json:"reason"`
	Error           *errInfo                 `json:"error,omitempty"`
	Pin             map[string]interface{}   `json:"pin,omitempty"`
	Negative        []map[string]interface{} `json:"negative_tests,omitempty"`
	ConnectMs       float64                  `json:"connect_ms,omitempty"`
	Root            map[string]interface{}   `json:"root,omitempty"`
	Shares          []*shareResult           `json:"shares,omitempty"`
	LatencyAll      latency                  `json:"listing_latency_all"`
	ClientRequests  int                      `json:"client_level_requests"`
	MinGapS         float64                  `json:"min_gap_between_client_requests_s"`
	Budget          map[string]interface{}   `json:"host_budget"`
	SpyCalls        map[string]int           `json:"calls_reaching_the_protocol_client"`
	Tripwire        map[string]interface{}   `json:"read_only_proof"`
	Findings        []string                 `json:"findings,omitempty"`
	NFS             map[string]interface{}   `json:"nfs,omitempty"`
	MeasuredAt      string                   `json:"measured_at"`
	WritesPerformed int                      `json:"writes_performed"`
	NamesRecorded   bool                     `json:"names_recorded"`
	ContentStored   bool                     `json:"content_stored"`
	Limits          map[string]interface{}   `json:"limits"`
}

func newHostResult(n int, proto, pkg string) *hostResult {
	return &hostResult{Schema: "wp12-nas-client/1", Alias: fmt.Sprintf("Synology%d", n), Protocol: proto, ClientPackage: pkg, Chain: chainName,
		MeasuredAt: time.Now().UTC().Format(time.RFC3339),
		Limits:     map[string]interface{}{"max_depth": maxDepth, "max_entries": maxEntries, "max_listing_requests_per_share": maxListPerShare, "max_requests_per_second": 1, "read_sample_bytes": maxReadBytes}}
}

func (h *hostResult) finishRO(c client.Client, sp *spy, b *fabric.HostBudget) {
	// write-class probes through the LIVE chain; the tripwire below ReadOnly refuses anything that gets that far, so nothing can reach a NAS even if ReadOnly were defective
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	probes := map[string]error{
		"WriteFile":       c.WriteFile(ctx, "/ncl-readonly-probe", bytes.NewReader([]byte("x"))),
		"CreateDirectory": c.CreateDirectory(ctx, "/ncl-readonly-probe"),
		"DeleteFile":      c.DeleteFile(ctx, "/ncl-readonly-probe"),
		"DeleteDirectory": c.DeleteDirectory(ctx, "/ncl-readonly-probe"),
		"CopyFile":        c.CopyFile(ctx, "/ncl-readonly-probe", "/ncl-readonly-probe2"),
	}
	res := map[string]string{}
	allRefused := true
	for k, err := range probes {
		if errors.Is(err, decorators.ErrReadOnly) {
			res[k] = "refused_by_ReadOnly(ErrReadOnly)"
		} else {
			res[k] = fmt.Sprintf("NOT_REFUSED_BY_READONLY(%T)", err)
			allRefused = false
		}
	}
	sp.mu.Lock()
	trips := append([]string(nil), sp.trips...)
	sp.mu.Unlock()
	if trips == nil {
		trips = []string{}
	}
	h.Tripwire = map[string]interface{}{"probe_results": res, "all_probes_refused_by_ReadOnly": allRefused, "tripwire_hits_below_ReadOnly": len(trips), "tripwire_hit_methods": trips,
		"note": "a hit below ReadOnly would have been refused by the harness tripwire, never sent to the NAS"}
	if !allRefused || len(trips) > 0 {
		h.Findings = append(h.Findings, "DEFECT: decorators.ReadOnly let a write-class call through on the live chain")
	}
	h.WritesPerformed = 0
	gap, n := sp.minGapSeconds()
	h.MinGapS, h.ClientRequests = gap, n
	h.SpyCalls = map[string]int{}
	sp.mu.Lock()
	for k, v := range sp.calls {
		h.SpyCalls[k] = v
	}
	h.LatencyAll = lat(sp.listDur)
	sp.mu.Unlock()
	st := b.Stats()
	bb, _ := json.Marshal(st)
	var bm map[string]interface{}
	_ = json.Unmarshal(bb, &bm)
	bm["config"] = map[string]interface{}{"max_concurrent": 1, "max_req_per_sec": 1}
	h.Budget = bm
	if gap >= 0 && gap < 0.95 {
		h.Findings = append(h.Findings, fmt.Sprintf("DEFECT: observed gap %.3fs between client-level requests is below the 1 request/second budget", gap))
	}
}

func writeResult(t *testing.T, h *hostResult, n int) {
	out := os.Getenv("NCL_OUT")
	b, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		t.Errorf("marshal: %v", err)
		return
	}
	s := string(b)
	for _, v := range []string{os.Getenv("NCL_PW"), os.Getenv("NCL_USER")} {
		if len(v) >= 3 && strings.Contains(s, v) {
			t.Errorf("LEAK: a credential value is in the result of host %d; result NOT written", n)
			return
		}
	}
	for i := 1; i <= 7; i++ {
		if ip := os.Getenv("NCL_IP_" + strconv.Itoa(i)); ip != "" && strings.Contains(s, ip) {
			t.Errorf("LEAK: an address is in the result of host %d; result NOT written", n)
			return
		}
	}
	if err := os.WriteFile(fmt.Sprintf("%s/%s-%d.json", out, h.Protocol, n), b, 0o644); err != nil {
		t.Errorf("write: %v", err)
	}
}

// ---- credential resolvers: the secret comes from the environment of the container by NAME ----

type ftpRes struct{}

func (ftpRes) Resolve(context.Context, string) (*ftp.Credential, error) {
	return &ftp.Credential{Password: os.Getenv("NCL_PW")}, nil
}

type sftpRes struct{}

func (sftpRes) Resolve(context.Context, string) (*sftp.Credential, error) {
	return &sftp.Credential{Password: os.Getenv("NCL_PW")}, nil
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// ---- the shared share loop ----

func runShares(ctx context.Context, h *hostResult, c client.Client, sp *spy, smb map[string]map[string]interface{}) {
	root, err := c.ListDirectory(ctx, "/")
	if err != nil {
		h.Status, h.Reason, h.Error = "error", "root_listing_failed", ei(err, true)
		return
	}
	var names, shares []string
	dirs := 0
	for _, f := range root {
		if f == nil || f.Name == "" || f.Name == "." || f.Name == ".." {
			continue
		}
		names = append(names, f.Name)
		if f.IsDir {
			dirs++
			shares = append(shares, f.Name)
		}
	}
	sort.Strings(shares)
	h.Root = map[string]interface{}{"entries": len(names), "dirs": dirs, "names_sha256": namesSHA(append([]string(nil), names...))}
	for _, s := range shares {
		smbSt := ""
		if sm, ok := smb[s]; ok {
			smbSt, _ = sm["status"].(string)
		}
		denied := smbSt == "denied_or_error"
		sr := walkShare(ctx, c, sp, s, "/"+s, !denied)
		if denied {
			sr.ReadSkippedReason = "share is DENIED over SMB for this account (security finding): bounded listing sample only, no file read"
		}
		sr.SMB = smbCross(smb, sr)
		if denied && sr.Status == "listed" {
			h.Findings = append(h.Findings, fmt.Sprintf("SECURITY: share %q is denied over SMB (NT_STATUS_ACCESS_DENIED) but its content is listable over %s (%d entries sampled)", s, h.Protocol, sr.SampledEntries))
		}
		h.Shares = append(h.Shares, sr)
	}
	h.Status, h.Reason = "works", "connected, listed the root and sampled every share through the fabric chain"
}

func newBudget() *fabric.HostBudget {
	b, err := fabric.NewHostBudget(fabric.BudgetConfig{MaxConcurrent: 1, MaxReqPerSec: 1})
	if err != nil {
		panic(err)
	}
	return b
}

func hostFor(n int) string { return os.Getenv("NCL_IP_" + strconv.Itoa(n)) }

func negRec(name, expect string, err error, ok bool) map[string]interface{} {
	m := map[string]interface{}{"name": name, "expected": expect, "ok": ok}
	if err != nil {
		m["got"] = fmt.Sprintf("%T", err)
		m["fabric_class"] = fabric.Classify(err).String()
		m["message"] = scrub(err.Error())
	} else {
		m["got"] = "nil error (connected)"
	}
	return m
}

// ---- FTP / FTPS ----

func runFTP(t *testing.T, n int, tls bool) {
	proto := "ftp"
	if tls {
		proto = "ftps"
	}
	ref := os.Getenv("NCL_REF")
	h := newHostResult(n, proto, "pkg/ftp")
	host := hostFor(n)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	defer func() { writeResult(t, h, n) }()
	smb := smbShares(ref, n)
	base := func(store ftp.PinStore, mode string, trusted bool) *ftp.Config {
		return &ftp.Config{Host: host, Port: 21, Username: os.Getenv("NCL_USER"), CredentialRef: "nas", Path: "/", Resolver: ftpRes{}, TLSMode: mode, TrustedLAN: trusted,
			PinStore: store, DialTimeout: 15 * time.Second, IOTimeout: 30 * time.Second}
	}
	var main *ftp.Config
	if tls {
		fp := ""
		if m := loadJSON(fmt.Sprintf("%s/ftps-%d.json", ref, n)); m != nil {
			if tm, ok := m["tls"].(map[string]interface{}); ok {
				fp, _ = tm["cert_sha256"].(string)
			}
		}
		if fp == "" {
			// the survey found no FTPS service: one real connect attempt with an empty store records the client's refusal class
		}
		// negative tests first: no credential may be sent in any of them
		neg := []map[string]interface{}{}
		c1 := ftp.NewFTPClient(base(ftp.NewMemPinStore(), ftp.TLSExplicit, false))
		e1 := c1.Connect(ctx)
		var unk *ftp.UnknownCertError
		if fp != "" {
			neg = append(neg, negRec("unpinned_certificate_refused", "*ftp.UnknownCertError (connection refused, no credential sent)", e1, errors.As(e1, &unk)))
		} else {
			neg = append(neg, negRec("service_absent_connect", "connection error (no FTPS service)", e1, e1 != nil))
		}
		_ = c1.Disconnect(ctx)
		if fp != "" {
			sleepCtx(ctx, time.Second)
			bogus := ftp.NewMemPinStore()
			_ = bogus.Record(ftp.HostPort(host, 21), ftp.CertPin{Fingerprint: "SHA256:" + strings.Repeat("AA:", 31) + "AA", ServerName: host, ConfirmedBy: "harness negative control"})
			c2 := ftp.NewFTPClient(base(bogus, ftp.TLSExplicit, false))
			e2 := c2.Connect(ctx)
			var mm *ftp.CertMismatchError
			neg = append(neg, negRec("changed_certificate_refused", "*ftp.CertMismatchError (connection refused, no credential sent)", e2, errors.As(e2, &mm)))
			_ = c2.Disconnect(ctx)
			sleepCtx(ctx, time.Second)
			_, e3 := ftp.Pin(ctx, ftp.NewMemPinStore(), host, 21, ftp.Confirmation{Fingerprint: strings.Repeat("ab", 32), ConfirmedBy: "harness negative control"})
			neg = append(neg, negRec("pin_with_wrong_confirmation_refused", "ErrPinNotConfirmed (nothing recorded)", e3, errors.Is(e3, ftp.ErrPinNotConfirmed)))
			sleepCtx(ctx, time.Second)
		}
		c4 := ftp.NewFTPClient(base(nil, ftp.TLSNone, false))
		e4 := c4.Connect(ctx)
		neg = append(neg, negRec("cleartext_without_trusted_lan_refused", "ftp.ErrClearTextRefused (before any network I/O)", e4, errors.Is(e4, ftp.ErrClearTextRefused)))
		h.Negative = neg
		if fp == "" {
			if e1 != nil {
				classifyConnectFailure(h, e1)
				if h.Status == "refused" {
					h.Reason += "; no survey certificate fingerprint exists to pin"
				}
			} else {
				h.Status, h.Reason = "blocked", "the host answered but the survey recorded no certificate fingerprint: pinning cannot be simulated"
			}
			return
		}
		store := ftp.NewMemPinStore()
		pin, perr := ftp.Pin(ctx, store, host, 21, ftp.Confirmation{Fingerprint: fp, ConfirmedBy: confirmedBy})
		if perr != nil {
			h.Pin = map[string]interface{}{"source": "survey_recorded_certificate_sha256", "outcome": "pin_refused", "error": ei(perr, true)}
			h.Status, h.Reason, h.Error = "refused", "the certificate the server presents now differs from (or could not be matched to) the survey-recorded fingerprint: refusal reported, never bypassed", ei(perr, true)
			return
		}
		h.Pin = map[string]interface{}{"source": "survey_recorded_certificate_sha256", "outcome": "pinned", "owner_confirmation": "SIMULATED by the recorded survey fingerprint (wp12 nas-protocols), not an out-of-band owner step",
			"fingerprint_sha256": strings.TrimPrefix(pin.Fingerprint, "SHA256:"), "server_name_kind": map[bool]string{true: "ip_matches_certificate", false: "certificate_dns_name"}[pin.ServerName == host]}
		main = base(store, ftp.TLSExplicit, false)
	} else {
		// plain FTP on the trusted LAN (the task authorises it; credentials travel in clear text on the LAN): first the refusal without trusted_lan
		neg := []map[string]interface{}{}
		c4 := ftp.NewFTPClient(base(nil, ftp.TLSNone, false))
		e4 := c4.Connect(ctx)
		neg = append(neg, negRec("cleartext_without_trusted_lan_refused", "ftp.ErrClearTextRefused (before any network I/O)", e4, errors.Is(e4, ftp.ErrClearTextRefused)))
		h.Negative = neg
		main = base(nil, ftp.TLSNone, true)
		h.Pin = map[string]interface{}{"outcome": "not_applicable_cleartext_trusted_lan", "note": "clear-text FTP on the LAN is the task's explicit scope; the credential travels unencrypted"}
	}
	b := newBudget()
	raw := ftp.NewFTPClient(main)
	c, sp, err := buildChain(raw, b)
	if err != nil {
		h.Status, h.Reason, h.Error = "error", "chain_build_failed", ei(err, true)
		return
	}
	t0 := time.Now()
	cerr := c.Connect(ctx)
	h.ConnectMs = round(float64(time.Since(t0).Microseconds())/1000, 1)
	if cerr != nil {
		classifyConnectFailure(h, cerr)
		h.finishRO(c, sp, b)
		return
	}
	defer c.Disconnect(context.Background())
	if terr := c.TestConnection(ctx); terr != nil {
		h.Findings = append(h.Findings, "DEFECT: TestConnection failed after a successful Connect: "+scrub(terr.Error()))
	}
	h.Findings = append(h.Findings, fmt.Sprintf("info: ftp client degraded (no MLSD) = %v", raw.Degraded()))
	runShares(ctx, h, c, sp, smb)
	h.finishRO(c, sp, b)
}

func classifyConnectFailure(h *hostResult, err error) {
	h.Error = ei(err, true)
	var ne interface{ Timeout() bool }
	switch {
	case strings.Contains(strings.ToLower(err.Error()), "refused"):
		h.Status, h.Reason = "refused", "connection refused by the host (service not offered)"
	case errors.As(err, &ne) && ne.Timeout():
		h.Status, h.Reason = "refused", "connection timed out (service not reachable)"
	default:
		h.Status, h.Reason = "error", "connect failed; see error"
	}
}

// ---- SFTP ----

func runSFTP(t *testing.T, n int) {
	ref := os.Getenv("NCL_REF")
	h := newHostResult(n, "sftp", "pkg/sftp")
	host := hostFor(n)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	defer func() { writeResult(t, h, n) }()
	smb := smbShares(ref, n)
	cfgOf := func(store sftp.PinStore) *sftp.Config {
		return &sftp.Config{Host: host, Port: 22, Username: os.Getenv("NCL_USER"), CredentialRef: "nas", Root: "/", Resolver: sftpRes{}, PinStore: store, DialTimeout: 20 * time.Second}
	}
	var fps []string
	if m := loadJSON(fmt.Sprintf("%s/sftp-%d.json", ref, n)); m != nil {
		if arr, ok := m["host_keys"].([]interface{}); ok {
			for _, a := range arr {
				if km, ok := a.(map[string]interface{}); ok {
					if f, _ := km["fingerprint_sha256"].(string); f != "" {
						fps = append(fps, f)
					}
				}
			}
		}
	}
	neg := []map[string]interface{}{}
	c1 := sftp.NewSFTPClient(cfgOf(sftp.NewMemPinStore()))
	e1 := c1.Connect(ctx)
	var unk *sftp.UnknownHostKeyError
	if len(fps) > 0 {
		neg = append(neg, negRec("unpinned_host_key_refused", "*sftp.UnknownHostKeyError (handshake aborted, no credential sent)", e1, errors.As(e1, &unk)))
	} else {
		neg = append(neg, negRec("service_absent_connect", "connection error (no SSH service)", e1, e1 != nil))
	}
	_ = c1.Disconnect(ctx)
	if len(fps) == 0 {
		h.Negative = neg
		if e1 != nil {
			classifyConnectFailure(h, e1)
			if h.Status == "refused" {
				h.Reason += "; no survey host key exists to pin"
			}
		} else {
			h.Status, h.Reason = "blocked", "the host answered but the survey recorded no host key: pinning cannot be simulated"
		}
		return
	}
	sleepCtx(ctx, time.Second)
	bogus := sftp.NewMemPinStore()
	_ = bogus.Record(sftp.HostPort(host, 22), sftp.HostKeyPin{KeyType: "ssh-ed25519", Fingerprint: "SHA256:" + strings.Repeat("A", 43), ConfirmedBy: "harness negative control"})
	c2 := sftp.NewSFTPClient(cfgOf(bogus))
	e2 := c2.Connect(ctx)
	var mm *sftp.HostKeyMismatchError
	neg = append(neg, negRec("changed_host_key_refused", "*sftp.HostKeyMismatchError (handshake aborted, no credential sent)", e2, errors.As(e2, &mm)))
	_ = c2.Disconnect(ctx)
	sleepCtx(ctx, time.Second)
	_, e3 := sftp.Pin(ctx, sftp.NewMemPinStore(), host, 22, sftp.Confirmation{Owner: "harness negative control", Fingerprint: "SHA256:" + strings.Repeat("B", 43)})
	neg = append(neg, negRec("pin_with_wrong_confirmation_refused", "sftp.ErrPinNotConfirmed (nothing recorded)", e3, errors.Is(e3, sftp.ErrPinNotConfirmed)))
	h.Negative = neg
	sleepCtx(ctx, time.Second)
	store := sftp.NewMemPinStore()
	var pinned, refused []map[string]interface{}
	for _, fp := range fps {
		p, perr := sftp.Pin(ctx, store, host, 22, sftp.Confirmation{Owner: confirmedBy, Fingerprint: fp})
		if perr != nil {
			refused = append(refused, map[string]interface{}{"fingerprint_sha256": fp, "error": ei(perr, true)})
			continue
		}
		pinned = append(pinned, map[string]interface{}{"key_type": p.KeyType, "fingerprint_sha256": p.Fingerprint})
		sleepCtx(ctx, time.Second)
	}
	h.Pin = map[string]interface{}{"source": "survey_recorded_host_key_fingerprints", "owner_confirmation": "SIMULATED by the recorded survey fingerprints (wp12 nas-protocols), not an out-of-band owner step",
		"pinned": pinned, "refused": refused}
	if len(pinned) == 0 {
		h.Status, h.Reason = "refused", "no recorded host key matches what the server presents now: refusal reported, never bypassed"
		return
	}
	b := newBudget()
	raw := sftp.NewSFTPClient(cfgOf(store))
	c, sp, err := buildChain(raw, b)
	if err != nil {
		h.Status, h.Reason, h.Error = "error", "chain_build_failed", ei(err, true)
		return
	}
	t0 := time.Now()
	cerr := c.Connect(ctx)
	h.ConnectMs = round(float64(time.Since(t0).Microseconds())/1000, 1)
	if cerr != nil {
		classifyConnectFailure(h, cerr)
		h.finishRO(c, sp, b)
		return
	}
	defer c.Disconnect(context.Background())
	if terr := c.TestConnection(ctx); terr != nil {
		h.Findings = append(h.Findings, "DEFECT: TestConnection failed after a successful Connect: "+scrub(terr.Error()))
	}
	runShares(ctx, h, c, sp, smb)
	h.finishRO(c, sp, b)
}

// ---- NFS ----

func runNFS(t *testing.T, n int) {
	ref := os.Getenv("NCL_REF")
	h := newHostResult(n, "nfs", "pkg/nfs3")
	host := hostFor(n)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	defer func() { writeResult(t, h, n) }()
	smb := smbShares(ref, n)
	mk := func(export string, priv bool) *nfs3.Client {
		c, err := nfs3.New(nfs3.Config{Host: host, Export: export, TryPrivilegedPort: priv, MaxRetries: -1, DialTimeout: 8 * time.Second, CallTimeout: 10 * time.Second, JukeboxRetries: -1})
		if err != nil {
			panic(err)
		}
		return c
	}
	nf := map[string]interface{}{}
	h.NFS = nf
	exps, eerr := mk("/volume1", false).Exports(ctx)
	if eerr != nil {
		nf["exports_call"] = ei(eerr, true)
	} else {
		var ps []string
		for _, e := range exps {
			ps = append(ps, e.Path)
		}
		sort.Strings(ps)
		nf["exports_call"] = "ok"
		nf["exports_listed"] = len(ps)
		nf["export_paths"] = ps
	}
	sleepCtx(ctx, time.Second)
	cand := []string{}
	seen := map[string]bool{}
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			cand = append(cand, p)
		}
	}
	for _, e := range exps {
		add(e.Path)
	}
	add("/volume1")
	add("/volume2")
	var shn []string
	for s, m := range smb {
		if st, _ := m["status"].(string); st == "listed" {
			shn = append(shn, s)
		}
	}
	sort.Strings(shn)
	for _, s := range shn {
		add("/volume1/" + s)
		add("/volume2/" + s)
	}
	add("/ncl-nonexistent-control")
	if len(cand) > 10 {
		cand = cand[:10]
	}
	var attempts []map[string]interface{}
	var okClient *nfs3.Client
	okPath := ""
	for _, p := range cand {
		c := mk(p, false)
		cctx, cc := context.WithTimeout(ctx, 40*time.Second)
		err := c.Connect(cctx)
		cc()
		a := map[string]interface{}{"export": p}
		if err == nil {
			a["result"] = "mounted"
			if okClient == nil {
				okClient, okPath = c, p
			} else {
				_ = c.Disconnect(ctx)
			}
		} else {
			a["result"] = "refused_or_failed"
			a["message"] = scrub(err.Error())
			var ae *nfs3.AccessError
			var me *nfs3.MountError
			var ne *nfs3.NFSError
			if errors.As(err, &ae) {
				a["access_error"] = map[string]interface{}{"layer": ae.Layer, "source_port": ae.SourcePort, "privileged_port_used": ae.Privileged}
			}
			if errors.As(err, &me) {
				a["mount_status"] = me.Status
			}
			if errors.As(err, &ne) {
				a["nfs_status"] = ne.Status
			}
			a["privileged_port_unavailable"] = errors.Is(err, nfs3.ErrPrivilegedPortUnavailable)
			a["type"] = fmt.Sprintf("%T", err)
		}
		attempts = append(attempts, a)
		if okClient != nil {
			break
		}
		sleepCtx(ctx, time.Second)
	}
	nf["mount_attempts"] = attempts
	nf["mount_attempts_try_privileged_port"] = false
	if okClient == nil && len(attempts) > 0 {
		// ONE extra attempt with TryPrivilegedPort: does the client keep the server's refusal when the reserved-port retry cannot be made (this container has no CAP_NET_BIND_SERVICE)?
		sleepCtx(ctx, time.Second)
		first := attempts[0]["export"].(string)
		cctx, cc := context.WithTimeout(ctx, 40*time.Second)
		perr := mk(first, true).Connect(cctx)
		cc()
		pa := map[string]interface{}{"export": first, "try_privileged_port": true}
		var pae *nfs3.AccessError
		var pme *nfs3.MountError
		if perr != nil {
			pa["message"] = scrub(perr.Error())
			pa["is_access_error"] = errors.As(perr, &pae)
			pa["has_mount_status"] = errors.As(perr, &pme)
			pa["privileged_port_unavailable"] = errors.Is(perr, nfs3.ErrPrivilegedPortUnavailable)
		} else {
			pa["result"] = "mounted"
		}
		nf["privileged_retry_attempt"] = pa
		_, firstWasAccess := attempts[0]["access_error"]
		if perr != nil && firstWasAccess && errors.Is(perr, nfs3.ErrPrivilegedPortUnavailable) && !errors.As(perr, &pme) {
			h.Findings = append(h.Findings, "DEFECT: nfs3.Client.Connect with TryPrivilegedPort returns only the error of the reserved-port retry (cannot bind a privileged source port) and DISCARDS the server's original refusal (AccessError / MNT3ERR_ACCES observed without TryPrivilegedPort): the operator is told about a local port problem instead of the export rule that actually refused")
		}
	}
	if okClient == nil {
		denied := 0
		for _, a := range attempts {
			if _, ok := a["access_error"]; ok {
				denied++
			}
		}
		h.Status = "blocked"
		h.Reason = fmt.Sprintf("every MNT attempt refused (%d of %d with the client's AccessError): the NFS export rules do not admit this machine's address; the owner must add an NFS permission rule. This is the server's decision, not a client failure", denied, len(attempts))
		return
	}
	b := newBudget()
	c, sp, err := buildChain(okClient, b)
	if err != nil {
		h.Status, h.Reason, h.Error = "error", "chain_build_failed", ei(err, true)
		return
	}
	defer c.Disconnect(context.Background())
	nf["mounted_export"] = okPath
	root := walkShare(ctx, c, sp, okPath, "/", true)
	root.SMB = map[string]interface{}{"note": "NFS export root, not comparable to a single SMB share"}
	h.Shares = append(h.Shares, root)
	h.Status, h.Reason = "works", "mounted an export and sampled it through the fabric chain"
	h.finishRO(c, sp, b)
}

// ---- entry ----

func TestNASClients(t *testing.T) {
	proto, hosts, out := os.Getenv("NCL_PROTO"), os.Getenv("NCL_HOSTS"), os.Getenv("NCL_OUT")
	if proto == "" || hosts == "" || out == "" || os.Getenv("NCL_PW") == "" {
		t.Skip("NCL_PROTO/NCL_HOSTS/NCL_OUT/NCL_PW not set")
	}
	var wg sync.WaitGroup
	for _, hs := range strings.Split(hosts, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(hs))
		if err != nil || n < 1 || n > 7 || hostFor(n) == "" {
			t.Fatalf("bad host %q", hs) // main goroutine
		}
		wg.Add(1)
		go func(n int) { // one goroutine per host: a host is always measured sequentially, hosts are independent machines
			defer wg.Done()
			switch proto {
			case "ftps":
				runFTP(t, n, true)
			case "ftp":
				runFTP(t, n, false)
			case "sftp":
				runSFTP(t, n)
			case "nfs":
				runNFS(t, n)
			default:
				t.Errorf("unknown protocol %q", proto)
			}
		}(n)
	}
	wg.Wait()
}
