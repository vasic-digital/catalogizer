WIZENV-1 (PROVISIONAL). Reproduce-first result:
- Leak REPRODUCED at the service layer with the real collectSystemInfo and real env values (red_prefix.txt): deny-list on password/secret/token passed *_API_KEY, *_DSN, *_BEARER.
- HTTP exposure NOT reproducible: ConfigurationWizardService has zero production constructors (reachability.txt, control needle >0 in tests); the routed /configuration/wizard/* handlers use ConfigurationService via an adapter. So not a live rc.1 leak; latent defect, fixed defensively.
- Fix: allow-list (safeEnvironment, wizardSafeEnvKeys). Green: green_fixed.txt (count=3). Mutant: restoring the deny-list fails the test (red_prefix.txt).
- No other os.Environ dump exists in non-test Go code. Values never printed (sentinel redacted).
Honest limit: no real-router test is possible because no route reaches the type; if it is ever wired, this class test already guards the data.
