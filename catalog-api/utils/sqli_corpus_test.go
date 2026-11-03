package utils

import "testing"

// sqliCorpus is the 51-payload SQL injection corpus (classic auth-bypass,
// union, stacked, blind, time-based, error-based and comment-terminator
// families). Entries are delivered as the validator sees them: decoded JSON
// string values (JSON bodies are not URL-decoded, so percent-encoded
// payloads are out of scope for the detector and are listed separately).
var sqliCorpus = []string{
	`' OR '1'='1`, `' OR 1=1 --`, `' OR 1=1#`, `' OR 1=1/*`, `admin' --`, `admin' #`,
	`admin'/*`, `" OR "1"="1`, `" OR 1=1 --`, `') OR ('1'='1`, `') OR 1=1 --`,
	`1 OR 1=1`, `1' OR '1'='1' --`, `' OR 'a'='a`, `' or 1=1 limit 1 --`,
	`' UNION SELECT NULL--`, `' UNION SELECT username, password FROM users--`,
	`1 UNION ALL SELECT 1,2,3`, `1' UNION/**/SELECT/**/1`, `x' UNION SELECT 1 #`,
	`'; DROP TABLE users; --`, `1; DROP TABLE users`, `x';SHUTDOWN`, `x'; SHUTDOWN --`,
	`1; EXEC xp_cmdshell('dir')`, `'; EXEC master..xp_cmdshell 'ping'`,
	`'; INSERT INTO users VALUES ('a','b'); --`, `'; UPDATE users SET admin=1; --`,
	`'; DELETE FROM users; --`, `1; SELECT pg_sleep(5)`, `1' AND SLEEP(5) --`,
	`'; WAITFOR DELAY '0:0:5'--`, `1 AND BENCHMARK(1000000,MD5(1))`,
	`1' AND (SELECT COUNT(*) FROM users) > 0 --`, `1 AND extractvalue(1,concat(0x7e,version()))`,
	`1 AND updatexml(1,concat(0x7e,user()),1)`, `1' AND 1=CONVERT(int,(SELECT @@version))--`,
	`1' ORDER BY 10--`, `1' ORDER BY 1#`, `' GROUP BY a HAVING 1=1 --`,
	`1' AND ASCII(SUBSTRING((SELECT password FROM users LIMIT 1),1,1))>64 --`,
	`'||(SELECT '')||'`, `' AND 1=1 --`, `' AND 'x'='x`, `1 AND 1=1`, `1 AND 2>1`,
	`'; SELECT LOAD_FILE('/etc/passwd'); --`, `' INTO OUTFILE '/tmp/x'--`,
	`1 PROCEDURE ANALYSE()`, `'; COPY users TO '/tmp/x'; --`, `' OR ''='`,
}

func TestContainsSQLInjection_Corpus51(t *testing.T) {
	if len(sqliCorpus) != 51 {
		t.Fatalf("corpus size %d, want 51", len(sqliCorpus))
	}
	var miss []string
	for _, p := range sqliCorpus {
		if !ContainsSQLInjection(p) {
			miss = append(miss, p)
		}
	}
	for _, m := range miss {
		t.Errorf("missed: %q", m)
	}
}

// Negative controls: legitimate text that must never be blocked.
func TestContainsSQLInjection_CorpusNegativeControls(t *testing.T) {
	for _, in := range []string{
		"Tom's #1 hits", "O'Brien", "rock 'n' roll", "AC/DC - Back in Black", "Q&A: select your favourite",
		"order by date", "Guns N' Roses #2", "price 5-10 dollars", "a -- b", "C# in depth", "tag #music #rock",
		"Union Station", "Select Committee report", "Drop the beat", "Update 2.0 notes", "Don't stop; believe",
		"shutdown sequence", "Insert coin", "Delete key", "Copy of Movie", "limit 5 per user",
		"Movie 1: The Start; Movie 2: The End", "Chapter 3; verse 4", "1 or 2 items", "x and y", "it's 5 > 3",
		"Fast & Furious", "Drop Dead Gorgeous", "Into the Wild", "procedure manual", "Loop; sleep well",
		"name' with apostrophe", "100% sure", "rock'n'roll", "Mr. 'Smith'", "A/B test", "What's up?",
	} {
		if ContainsSQLInjection(in) {
			t.Errorf("false positive: %q", in)
		}
	}
}

// Review r1 additions: attacks the first detector missed ...
func TestContainsSQLInjection_ReviewR1Attacks(t *testing.T) {
	for _, p := range []string{
		`1 and substr(version(),1,1)='5'`, `1 AND randomblob(500000000)`, `1 AND load_extension('x')`,
		`' || '1'='1`, `1 order by 5`, `1' order by 3--`, `admin' #`, `1' ORDER BY 1#`, `1) --`, `x' -- `,
		`1 UNION SELECT NULL`, `1 UNION SELECT user()`, `1 AND SLEEP(5)`, `1;SELECT pg_sleep(10)`, `1 AND ASCII(SUBSTRING((SELECT 1),1,1))>1`,
	} {
		if !ContainsSQLInjection(p) {
			t.Errorf("missed: %q", p)
		}
	}
}

// ... and legitimate text it blocked.
func TestContainsSQLInjection_ReviewR1Legit(t *testing.T) {
	for _, p := range []string{
		"Ep. 7 #", "Season 2 #", "Episode 3 -- The Return", "Best of 2020 -- 2021", "Back in Black (live) -- remaster",
		"Chapter 1 -- intro", "Movie (1999) -- director's cut", `"Quoted" -- review`, "Price: $10 -- sale",
		"the union select committee", "Sleep (2010 film)", "sleep(well) song", "Please order by popularity",
	} {
		if ContainsSQLInjection(p) {
			t.Errorf("false positive: %q", p)
		}
	}
}
