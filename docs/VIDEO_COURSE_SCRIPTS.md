# Catalogizer Video Course Scripts

## Course Catalog

This document contains video course scripts for Catalogizer users, developers, and integrators.

**Total Duration:** ~4 hours across 3 courses
**Courses:** 3 (User Onboarding, Developer Training, API Integration)
**Target Audience:** End users, Developers, Integrators

---

# COURSE 1: User Onboarding (5 videos - 32 minutes)

## Video 1.1: Getting Started with Catalogizer (5 min)

### Learning Objectives
- Install Catalogizer
- Complete initial setup wizard
- Navigate the main dashboard

### Script

**[00:00-00:30] - Introduction**
```
Welcome to Catalogizer! I'm excited to show you how to organize
your entire media collection in one place.

Whether you have movies, TV shows, music, games, books, or comics -
Catalogizer handles it all automatically.
```

**[00:30-02:00] - Installation**
```
Let's start with installation. You have several options:

Option 1: Download from our website (catalogizer.io)
- Available for Windows, macOS, and Linux
- Just download and run the installer

Option 2: Use Docker
```bash
docker run -p 3000:3000 -p 8080:8080 vasicdigital/catalogizer
```

Option 3: Build from source
- Clone the repository
- Follow the README instructions

For this tutorial, I'll use the desktop installer.
```

**[02:00-04:00] - Initial Setup**
```
When you first launch Catalogizer, you'll see the setup wizard.

Step 1: Create your admin account
- Choose a username and secure password
- This account has full access to everything

Step 2: Configure your first storage location
- This is where your media files are stored
- Can be local folders, network drives, or cloud storage
- You can add more locations later

Step 3: Set your preferences
- Default language
- Theme (light/dark)
- Media types to enable

**[04:00-05:00] - Dashboard Overview**
```
Setup complete! Welcome to your dashboard.

Here's what you see:
- Top: Search bar and quick actions
- Left sidebar: Navigation between media types
- Main area: Overview cards and recent items
- Bottom: Storage usage and system status

Key features:
- Total items count updates automatically
- Recently added shows your newest media
- Storage usage helps you manage space
- Quick scan button for instant updates
```

---

## Video 1.2: Understanding Media Types (7 min)

### Script

**[00:00-01:00] - Introduction to Media Types**
```
Catalogizer supports 11 media types, each with specialized
handling and metadata sources.

Let's explore them one by one.
```

**[01:00-02:00] - Movies & TV Shows**
```
MOVIES:
- Automatic detection from filenames
- Metadata from TMDB, IMDB, Rotten Tomatoes
- Posters, backdrops, cast information
- Trailer links and ratings

TV SHOWS:
- Full season/episode organization
- Episode-specific metadata
- Watch status tracking per episode
- "Up next" recommendations

Both support:
- 4K, HDR, Dolby Vision detection
- Multiple audio/subtitle tracks
- File quality indicators
```

**[02:00-03:00] - Music**
```
MUSIC:
- Artist and album organization
- Track-level metadata
- Genre classification
- Playlist creation and management
- Integration with MusicBrainz and Last.fm

Special features:
- Album art fetching
- Lyrics support
- Audio quality indicators (FLAC, MP3 bitrate)
- Compilation handling
```

**[03:00-04:00] - Games & Software**
```
GAMES:
- PC games (Steam, GOG, Epic, etc.)
- Console games (with emulator support)
- Mobile games (APK/IPA tracking)
- Metadata from IGDB and Steam

SOFTWARE:
- Application tracking
- Version management
- License key storage (encrypted)
- Installation package organization
```

**[04:00-05:00] - Books & Comics**
```
BOOKS:
- E-books (EPUB, MOBI, PDF)
- Audiobooks with chapter support
- Metadata from OpenLibrary, Goodreads
- Reading progress tracking

COMICS:
- CBZ/CBR archive support
- Page-by-page navigation
- Series and volume organization
- Metadata from ComicVine
```

**[05:00-06:00] - Other Media Types**
```
OTHER SUPPORTED TYPES:

PODCASTS:
- RSS feed integration
- Episode downloads
- Listening progress

PHOTOS:
- EXIF data extraction
- Location mapping
- Album organization

DOCUMENTS:
- PDF, Word, Excel support
- Full-text search
- Tag-based organization

ANIME:
- Special anime metadata fields
- MyAnimeList integration
- Episode tracking

YOUTUBE VIDEOS:
- Channel subscriptions
- Playlist import
- Offline downloads
```

**[06:00-07:00] - Summary**
```
With 11 media types, Catalogizer is your complete digital
library solution.

Each type has:
- Automatic metadata fetching
- Specialized organization
- Quality indicators
- Unique features tailored to the content

Next, let's see how to organize everything effectively.
```

---

## Video 1.3: Organizing Your Collection (6 min)

### Script

**[00:00-01:00] - Collections**
```
COLLECTIONS are the primary way to organize your media.

Think of them like playlists or folders, but more powerful.

Creating a collection:
1. Click "New Collection" button
2. Give it a name and description
3. Choose a cover image
4. Start adding items

Examples:
- "Favorite Movies"
- "Christmas Music"
- "Workout Playlist"
- "Retro Games"
```

**[01:00-02:30] - Smart Collections**
```
SMART COLLECTIONS automatically update based on rules.

Examples:
- "Unwatched Movies" - movies where play count = 0
- "4K Content" - files with 4K resolution
- "Recent Additions" - added in last 30 days
- "High Rated" - rating > 8.0

Setting up rules:
1. Select metadata field (year, rating, genre, etc.)
2. Choose operator (equals, greater than, contains)
3. Enter value
4. Combine multiple rules with AND/OR
```

**[02:30-04:00] - Tags and Labels**
```
TAGS provide flexible, multi-dimensional organization.

Unlike collections, items can have unlimited tags.

Creating tags:
- Use existing tags or create new ones
- Color-code for visual organization
- Nest tags hierarchically

Examples:
- Genre tags: #action, #comedy, #sci-fi
- Quality tags: #4K, #HDR, #remastered
- Personal tags: #favorite, #watch-again, #kids

Filtering:
- Click any tag to filter
- Combine multiple tags
- Exclude tags with NOT operator
```

**[04:00-05:30] - Favorites and Watchlists**
```
FAVORITES:
- Quick access from dashboard
- Heart icon on any item
- Separate favorites per media type

WATCHLIST:
- Items you want to watch/play/read
- Integration with external services
- Reminders for new releases

CUSTOM LISTS:
- Priority lists
- "Up next" queues
- Sharing lists with friends
```

**[05:30-06:00] - Search and Filters**
```
POWERFUL SEARCH:

Basic search:
- Type in search box
- Searches titles, descriptions, cast

Advanced filters:
- Year range
- Rating range
- File quality
- Added date
- Play count

Saved searches:
- Save filter combinations
- Quick access from sidebar
- Share with other users
```

---

## Video 1.4: Scanning and Auto-Detection (8 min)

### Script

**[00:00-01:30] - Starting a Scan**
```
The magic of Catalogizer is automatic detection.

To start scanning:
1. Click the "Scan" button
2. Select storage location
3. Choose media types to detect
4. Set options (deep scan, skip existing, etc.)

Real-time progress shows:
- Files analyzed
- Media found
- Detection confidence
- Current file being processed
```

**[01:30-03:00] - How Detection Works**
```
DETECTION PROCESS:

Step 1: Filename Analysis
- Pattern matching for titles
- Year extraction
- Quality indicators (1080p, BluRay, etc.)
- Episode/season detection

Step 2: File Analysis
- Media info extraction (codec, resolution)
- Duration calculation
- Checksum for duplicate detection
- Thumbnail generation

Step 3: Metadata Fetching
- Query online databases
- Match by title/year
- Download posters and artwork
- Get cast, crew, descriptions

Confidence score shows how sure Catalogizer is about each detection.
```

**[03:00-04:30] - Reviewing Results**
```
AFTER SCAN COMPLETES:

High confidence items (>80%):
- Automatically added to library
- Fully populated metadata
- Ready to use

Medium confidence (50-80%):
- Added to "Review" queue
- You confirm or correct
- One-click fixes

Low confidence (<50%):
- Flagged for manual entry
- All fields editable
- Search for correct metadata

The review interface shows:
- Suggested matches
- Confidence scores
- Alternative options
- Manual search capability
```

**[04:30-06:00] - Handling Duplicates**
```
DUPLICATE DETECTION:

Catalogizer automatically finds duplicates by:
- File checksum
- Title/year matching
- Similar filenames

Duplicate handling options:
- Keep best quality version
- Keep all versions
- Merge metadata
- Delete duplicates

Quality comparison shows:
- Resolution (4K vs 1080p)
- File size
- Codec (H.265 vs H.264)
- Audio tracks
```

**[06:00-07:30] - Scheduled and Automatic Scanning**
```
AUTOMATION OPTIONS:

Scheduled scans:
- Daily, weekly, or custom schedule
- Time-based (scan at 3 AM)
- Incremental (only new files)

Real-time monitoring:
- Watch folders for changes
- Auto-detect new files
- Immediate processing

Notifications:
- Email when scan completes
- Summary of new items found
- Alerts for issues

Settings:
- Exclude patterns (temp files, samples)
- Minimum file size
- File type whitelist
```

**[07:30-08:00] - Best Practices**
```
SCANNING BEST PRACTICES:

1. Organize files before scanning
   - Use consistent naming
   - Group related files
   - Remove samples and extras

2. Start with a subset
   - Test with one folder first
   - Verify detection accuracy
   - Adjust settings as needed

3. Regular maintenance
   - Weekly scans for new content
   - Monthly library cleanup
   - Review and merge duplicates
```

---

## Video 1.5: Playback and Streaming (6 min)

### Script

**[00:00-01:30] - Built-in Media Player**
```
BUILT-IN PLAYER features:

Video playback:
- Hardware acceleration
- Subtitle support (SRT, ASS, VTT)
- Audio track selection
- Playback speed (0.5x to 2x)
- Resume from last position

Music playback:
- Gapless playback
- Visualizations
- Playlist queue
- Shuffle and repeat

Reading:
- PDF viewer with search
- E-book reader with bookmarks
- Comic reader with zoom
```

**[01:30-03:00] - Streaming to Devices**
```
STREAMING OPTIONS:

Smart TVs:
- DLNA/UPnP support
- Samsung, LG, Sony, Roku
- Browse and play from TV interface

Mobile apps:
- iOS and Android apps
- Offline downloads
- Continue watching sync

Web browser:
- Access from any device
- No app installation needed
- Same interface as desktop

Casting:
- Chromecast support
- AirPlay for Apple devices
- Smart display integration
```

**[03:00-04:30] - Transcoding**
```
TRANSCODING features:

Automatic format conversion:
- Unsupported codecs → compatible formats
- High bitrate → bandwidth-optimized
- 4K → 1080p for mobile devices

Quality options:
- Original (direct stream, no conversion)
- High (1080p, high bitrate)
- Medium (720p, medium bitrate)
- Low (480p, low bitrate)

Hardware acceleration:
- Intel QuickSync
- NVIDIA NVENC
- AMD VCE
- Apple VideoToolbox

Settings:
- Per-device defaults
- Adaptive streaming (adjusts to bandwidth)
- Pre-transcode popular content
```

**[04:30-05:30] - Offline Access**
```
DOWNLOADS for offline:

Use cases:
- Travel without internet
- Commute on subway
- Reduce mobile data usage

Download options:
- Quality selection
- Single items or entire collections
- Auto-delete after watching
- Storage management

Sync features:
- Resume position syncs across devices
- Downloads on one device, continue on another
- Auto-download new episodes
```

**[05:30-06:00] - User Management**
```
MULTI-USER features:

User accounts:
- Separate libraries per user
- Individual watch histories
- Personalized recommendations
- Custom permissions

Sharing:
- Share specific collections
- Family groups
- Guest access with limits

Parental controls:
- Content ratings
- Time limits
- Allowed media types
- PIN protection

Congratulations! You now know the basics of Catalogizer.

Check out our advanced tutorials for:
- API integration
- Custom metadata providers
- Plugin development
- Server administration

Happy organizing!
```

---

# COURSE 2: Developer Training (8 videos - ~2 hours)

[Detailed developer training content would continue here...]

---

# COURSE 3: API Integration Guide (3 videos - ~45 min)

[Detailed API integration content would continue here...]

---

## Production Notes

### Recording Specifications
- **Resolution**: 1920x1080 (1080p minimum)
- **Frame Rate**: 30fps
- **Audio**: 48kHz, stereo, -16 LUFS
- **Format**: MP4 (H.264 codec)

### Visual Style
- Clean, modern UI recordings
- Highlight important elements with zoom/pan
- Consistent color scheme
- Professional but approachable tone

### Post-Production
- Captions for accessibility
- Chapter markers for navigation
- Code samples in description
- Links to related resources

### Distribution
- YouTube (primary)
- GitHub repository
- Documentation website
- Course platforms (Udemy, etc.)

---

**Last Updated**: 2026-03-22
**Version**: 1.0

---

## Module 1: Introduction (15 minutes)

### Learning Objectives
- Understand what autonomous QA testing is
- Learn the benefits over traditional QA
- Get an overview of HelixQA architecture

### Script

**[00:00-01:00] - Introduction & Hook**
```
"Welcome to the HelixQA Autonomous QA Session course. 

If you're tired of manually clicking through your application 
to find bugs, writing endless test scripts, or maintaining 
fragile test suites that break with every UI change - this 
course is for you.

Today I'm going to show you how to leverage Large Language 
Models to autonomously test your applications across multiple 
platforms - Android, Web, and Desktop - with minimal setup."
```

**[01:00-04:00] - What is Autonomous QA?**
```
"Traditional QA testing falls into three categories:

1. Manual Testing - Time-consuming, inconsistent, doesn't scale
2. Scripted Testing - Brittle, high maintenance, can't adapt to UI changes  
3. Record & Playback - Limited coverage, breaks easily

Autonomous QA is different. It uses AI to:
- Understand your application through documentation
- Intelligently navigate without brittle selectors
- Detect issues using visual and functional analysis
- Generate comprehensive bug reports automatically

Think of it as having a smart QA engineer that never gets tired, 
works 24/7, and can test on multiple platforms simultaneously."
```

**[04:00-08:00] - HelixQA Architecture Overview**
```
"HelixQA consists of four main phases:

Phase 1: Setup
- LLMsVerifier selects the best available LLM models
- DocProcessor extracts features from your documentation
- LLMOrchestrator initializes agent pools

Phase 2: Document-Driven Verification
- System navigates to documented features
- Verifies each feature works correctly
- Collects evidence along the way

Phase 3: Curiosity-Driven Exploration
- Explores unvisited parts of the app
- Tests edge cases
- Detects visual and functional bugs

Phase 4: Report Generation
- Creates detailed tickets for each issue
- Generates coverage reports
- Provides video evidence with timestamps

The system supports multiple LLM providers including 
Anthropic, OpenAI, Google, and more - automatically 
selecting the best model for each task."
```

**[08:00-12:00] - Live Demo Preview**
```
"Let me show you what a typical session looks like:

[Show screen recording]

In this example, HelixQA is testing a web application.
Notice how it:

1. Reads the README and docs to understand features
2. Navigates to the login page and tests authentication
3. Discovers the settings page wasn't documented
4. Finds a visual bug: truncated button text
5. Generates a detailed ticket with screenshot and fix suggestion

All of this happened automatically in about 5 minutes."
```

**[12:00-15:00] - Course Roadmap**
```
"In this course, we'll cover:

Module 1: Introduction (this video) - Overview and concepts
Module 2: Configuration - Setting up environment and agents
Module 3: Running Sessions - Executing tests and monitoring
Module 4: Advanced Features - Custom strategies and integrations
Module 5: Troubleshooting - Common issues and solutions

By the end, you'll be able to set up autonomous QA testing 
for your own projects and integrate it into your CI/CD pipeline.

Let's get started with configuration in Module 2."
```

---

## Module 2: Configuration (25 minutes)

### Learning Objectives
- Set up environment variables
- Configure LLM providers
- Install and configure CLI agents
- Understand strategy selection

### Script

**[00:00-03:00] - Prerequisites**
```
"Before we configure HelixQA, ensure you have:

1. Go 1.24 or later installed
   Check: go version

2. Git access to the Catalogizer repository

3. At least one LLM API key
   - Anthropic (recommended for vision)
   - OpenAI (GPT-4V)
   - Google (Gemini)

4. Optional but recommended:
   - ffmpeg for video recording
   - OpenCV for enhanced vision

For this tutorial, I'll use Anthropic's Claude as the 
primary LLM since it has excellent vision capabilities."
```

**[03:00-10:00] - Environment Configuration**
```
"Let's create the .env file:

[Show terminal]

cp .env.example .env
nano .env

The most important variables are:

# Master switches
HELIX_AUTONOMOUS_ENABLED=true
HELIX_AUTONOMOUS_PLATFORMS=desktop,web
HELIX_AUTONOMOUS_TIMEOUT=2h

# LLM Provider - minimum one required
ANTHROPIC_API_KEY=sk-ant-...

# Optional: Add more providers for redundancy
OPENAI_API_KEY=sk-...
GOOGLE_API_KEY=...

# CLI Agents
HELIX_AGENTS_ENABLED=opencode,claude-code
HELIX_AGENT_POOL_SIZE=3
HELIX_AGENT_TIMEOUT=60s

For testing desktop applications on Linux:
HELIX_DESKTOP_DISPLAY=:0
HELIX_DESKTOP_PROCESS=myapp

For web testing:
HELIX_WEB_URL=http://localhost:3000
HELIX_WEB_BROWSER=chromium

Save the file and we're ready for the next step."
```

**[10:00-16:00] - Installing CLI Agents**
```
"HelixQA supports multiple CLI agents. Let's install them:

1. OpenCode (recommended - multi-provider)
   go install github.com/opencode-ai/opencode@latest

2. Claude Code (Anthropic's official CLI)
   npm install -g @anthropic-ai/claude-code

3. Gemini CLI (Google)
   npm install -g @google/gemini-cli

Verify installations:
which opencode
which claude
which gemini

Each agent has different strengths:
- OpenCode: Flexible, supports multiple LLM backends
- Claude Code: Native Anthropic support, excellent vision
- Gemini: Largest context window, good for large codebases

You can mix and match agents. HelixQA will select the 
best one for each task based on your strategy configuration."
```

**[16:00-22:00] - Strategy Configuration**
```
"The strategy determines how LLMs are selected and scored.

In your .env, set:
LLMSVERIFIER_STRATEGY=helix-qa

Available strategies:

1. helix-qa (default) - Balanced for autonomous testing
   Vision: 25%, Speed: 25%, Quality: 30%, Cost: 10%, Reliability: 10%

2. speed - Fast responses, lower quality
   Useful for quick smoke tests

3. quality - Highest quality, slower
   Use for thorough regression testing

4. vision - Prioritizes vision capabilities
   Essential for UI-heavy applications

5. cost - Budget-conscious
   Minimizes API costs

For most QA testing, helix-qa is optimal. It balances:
- Vision capability for UI analysis
- Speed for responsive interactions  
- Quality for accurate bug detection
- Cost efficiency

You can customize weights if needed by implementing 
a custom strategy - we'll cover that in Module 4."
```

**[22:00-25:00] - Configuration Verification**
```
"Let's verify our configuration:

[Show terminal]

cd HelixQA
go build ./cmd/helixqa
./helixqa --help

Test configuration loading:
./helixqa validate --env ../.env

If everything is correct, you should see:
'Configuration valid: 3 agents configured, 2 platforms enabled'

Common issues:
- Missing API keys: Check .env file permissions (chmod 600)
- Agent not found: Add to PATH or specify full path
- Invalid strategy: Use one of the predefined strategy names

Once validation passes, you're ready to run your first 
autonomous session in Module 3."
```

---

## Module 3: Running Sessions (30 minutes)

### Learning Objectives
- Execute autonomous QA sessions
- Monitor progress in real-time
- Interpret session output
- Manage session lifecycle

### Script

**[00:00-05:00] - First Session Execution**
```
"Let's run your first autonomous QA session.

Prerequisites:
- Your application should be running
- Documentation exists (README, docs folder)
- Configuration from Module 2 is complete

Command structure:
./helixqa autonomous \\
  --project /path/to/project \\
  --platforms desktop \\
  --env ../.env \\
  --output ./qa-results \\
  --timeout 30m

[Show terminal executing command]

Watch the output:
- Phase indicators: [setup] [doc-driven] [curiosity] [report]
- Progress tracking: Feature X/Y
- Issue detection: Real-time alerts
- Timeline events: Actions being taken

The session runs through 4 phases automatically.
Let me explain each one..."
```

**[05:00-12:00] - Understanding Session Phases**
```
"Phase 1: Setup (30-60 seconds)

You'll see:
[setup] LLMsVerifier: Scoring models...
[setup] Selected: claude-3.5-sonnet (score: 0.87)
[setup] DocProcessor: Extracting features...
[setup] Found: 42 features from 12 documents
[setup] LLMOrchestrator: Spawning agents...
[setup] 3 agents ready
[setup] Recording started

This phase:
- Ranks available LLMs
- Parses documentation
- Initializes agent pool
- Starts video recording

Phase 2: Doc-Driven Verification

[doc-driven][desktop] Verifying: 'User Login' (1/42)
[doc-driven][desktop]   ✓ Navigate to /login
[doc-driven][desktop]   ✓ Enter test credentials  
[doc-driven][desktop]   ✓ Submit form
[doc-driven][desktop]   ✓ Verify dashboard loads
[doc-driven][desktop] Feature verified: user-login

The system:
- Reads feature descriptions from docs
- Uses LLM to generate test steps
- Executes navigation actions
- Verifies expected outcomes
- Captures before/after screenshots

Phase 3: Curiosity-Driven Exploration

[curiosity][desktop] Exploring: Settings page
[curiosity][desktop]   Testing: Form validation
[curiosity][desktop]   Issue: Empty input accepted (medium)
[curiosity][desktop]   Evidence: screenshot-034.png

This phase:
- Discovers unvisited screens
- Tests edge cases
- Detects undocumented bugs
- Continues until budget exhausted

Phase 4: Report Generation

[report] Stopping recordings...
[report] Aggregating results...
[report] Coverage: 95.2%
[report] Issues: 3 found
[report] Writing reports to ./qa-results/

Generates:
- Markdown report with summary
- Individual tickets per issue
- Timeline with video timestamps
- Coverage analysis"
```

**[12:00-20:00] - Monitoring & Control**
```
"While a session runs, you can:

1. View Progress
   ./helixqa status --session-id abc123
   
   Shows:
   - Current phase
   - Features verified/total
   - Issues found
   - Time elapsed/remaining

2. Pause/Resume
   ./helixqa pause --session-id abc123
   ./helixqa resume --session-id abc123
   
   Useful for:
   - Debugging application issues
   - System maintenance
   - API rate limit management

3. Cancel
   ./helixqa cancel --session-id abc123
   
   Gracefully stops the session and generates 
   partial reports with collected evidence.

4. Real-time Logs
   tail -f qa-results/session.log
   
   Shows detailed LLM interactions, 
   navigation steps, and error traces.

[Show each command in terminal]

Monitoring Best Practices:
- Check progress after first 5 minutes
- Review issues immediately if critical severity
- Monitor API usage for cost control
- Ensure video recording is working"
```

**[20:00-27:00] - Interpreting Results**
```
"After session completion, check:

1. Executive Summary (qa-report.md)
   
   Coverage: 95.2%
   Duration: 1h 23m
   Platforms: desktop, web
   
   Issues by Severity:
   - Critical: 0
   - High: 1
   - Medium: 2
   - Low: 0

2. Individual Tickets (tickets/HQA-*.md)

   Example ticket:
   # HQA-0001: Login button not responding
   
   Severity: High | Platform: Desktop
   
   Steps to Reproduce:
   1. Navigate to /login
   2. Enter credentials
   3. Click login button
   
   Expected: Dashboard loads
   Actual: Nothing happens
   
   Evidence:
   - Screenshot: screenshots/001.png
   - Video: videos/desktop.mp4 @ 05:23
   
   LLM Analysis:
   Button event handler not bound. 
   Check login.js line 45.

3. Timeline (timeline.json)
   
   Chronological event log:
   - 00:00 - Session start
   - 02:15 - Feature verification: login
   - 05:23 - Issue detected: button
   - 07:45 - Feature verification: signup
   
4. Coverage Report
   
   Features Verified: 40/42
   - login: ✓
   - signup: ✓
   - password-reset: ✓
   - profile: ⚠ skipped (timeout)
   - settings: ✓

Interpreting Coverage:
- >90%: Excellent, most features tested
- 70-90%: Good, some features missed
- <70%: Review documentation completeness"
```

**[27:00-30:00] - Session Management**
```
"Managing multiple sessions:

List active sessions:
./helixqa list --status running

Compare sessions:
./helixqa compare session1 session2

Archive old results:
./helixqa archive --before 2026-03-01

Integration with CI/CD:

GitLab CI example:
```yaml
qa-autonomous:
  script:
    - ./helixqa autonomous \\
        --project . \\
        --platforms web \\
        --output qa-results
    - ./scripts/parse-qa-results.sh
  artifacts:
    paths:
      - qa-results/
```

Parse results script:
#!/bin/bash
CRITICAL=$(jq '.issues.critical' qa-results/summary.json)
if [ "$CRITICAL" -gt 0 ]; then
  echo "Critical issues found!"
  exit 1
fi

This fails the pipeline if critical issues exist.

In Module 4, we'll cover advanced features like 
custom strategies and multi-platform testing."
```

---

## Module 4: Advanced Features (35 minutes)

### Learning Objectives
- Implement custom verification strategies
- Configure multi-platform testing
- Use LLM navigation effectively
- Integrate with existing workflows

### Script

**[00:00-08:00] - Custom Strategies**
```
"When predefined strategies aren't sufficient, 
create custom ones.

Example: Mobile-First Strategy

Create pkg/strategy/mobile.go:

package strategy

import (
    "context"
    "digital.vasic.llmsverifier/pkg/strategy"
)

type MobileFirstStrategy struct {
    base strategy.VerificationStrategy
}

func (s *MobileFirstStrategy) Score(ctx context.Context, 
    model strategy.ModelInfo) (strategy.StrategyScore, error) {
    
    baseScore, _ := s.base.Score(ctx, model)
    
    // Bonus for mobile-optimized models
    if model.AvgLatencyMs < 1000 {
        baseScore.Overall += 0.1
    }
    
    // Prefer models with good mobile support
    if model.Provider == "anthropic" {
        baseScore.Overall += 0.05
    }
    
    return baseScore, nil
}

func (s *MobileFirstStrategy) Name() string {
    return "mobile-first"
}

Register the strategy:

strategy.Register("mobile-first", func() strategy.VerificationStrategy {
    return &MobileFirstStrategy{
        base: strategy.NewDefaultStrategy(),
    }
})

Use in .env:
LLMSVERIFIER_STRATEGY=mobile-first

Other use cases:
- Compliance-focused (audit trail requirements)
- Performance-critical (sub-second responses)
- Cost-optimized (cheapest viable model)
- Region-specific (data residency)"
```

**[08:00-16:00] - Multi-Platform Testing**
```
"Testing across platforms simultaneously:

Configuration:
HELIX_AUTONOMOUS_PLATFORMS=android,web,desktop
HELIX_AGENT_POOL_SIZE=6  # 2 per platform

Parallel Execution:

The system spawns workers for each platform:

Worker 1: Android (emulator-5554)
  - ADB commands
  - Logcat monitoring
  - APK installation

Worker 2: Web (Playwright)
  - Chromium browser
  - Page navigation
  - Console log capture

Worker 3: Desktop (X11)
  - xdotool automation
  - Window management
  - Screenshot capture

Each worker:
- Has dedicated agent from pool
- Records separate video
- Generates platform-specific tickets
- Updates shared coverage map

Platform-Specific Configuration:

Android:
HELIX_ANDROID_DEVICE=emulator-5554
HELIX_ANDROID_PACKAGE=com.example.app

Web:
HELIX_WEB_URL=http://localhost:3000
HELIX_WEB_BROWSER=chromium

Desktop:
HELIX_DESKTOP_PROCESS=myapp
HELIX_DESKTOP_DISPLAY=:1

Cross-Platform Issues:

When an issue appears on multiple platforms:

HQA-0001: Login fails [Android, Web]
HQA-0002: Login fails [Desktop]

System detects similarity and links tickets.

[Show example multi-platform session]

Coverage aggregation:
Android: 42 features, 2 issues
Web: 40 features, 1 issue  
Desktop: 38 features, 3 issues
Overall: 95% coverage, 6 unique issues"
```

**[16:00-24:00] - Advanced Navigation**
```
"LLM Navigation Deep Dive:

The navigator uses a graph-based approach:

NavigationGraph {
  Screens: [Login, Dashboard, Settings]
  Transitions: {
    Login → Dashboard: {action: "login"}
    Dashboard → Settings: {action: "click-settings"}
  }
}

When targeting a screen:

1. Check if path exists in graph
   ShortestPath(Login, Settings) 
   → [Login → Dashboard → Settings]

2. If path unknown, infer with LLM:
   
   Prompt: "How do I reach Settings from Login?"
   LLM: "Click menu icon, then Settings"
   
3. Execute actions with verification
   - Take screenshot before
   - Perform action
   - Take screenshot after
   - Verify state changed

4. Update graph with new path

Edge Cases:

Dynamic UI (loading states):
- Wait for element with retry
- Use vision to detect ready state
- Timeout after configured limit

Unexpected dialogs:
- Detect popup/modal
- Handle or dismiss
- Log intervention

Navigation failures:
- Retry with alternative path
- Escalate to human if stuck
- Continue with other features

Optimization Tips:

1. Seed initial navigation graph
   - Manually define common paths
   - Reduces LLM calls
   - Faster initial navigation

2. Cache successful paths
   - Reuse across sessions
   - Updates with new discoveries

3. Prioritize high-value features
   - Critical user flows first
   - Peripheral features last

[Show navigation graph visualization]"
```

**[24:00-30:00] - Integration Patterns**
```
"Integrating HelixQA into your workflow:

1. Pre-Commit Hook

#!/bin/bash
# .git/hooks/pre-commit

./helixqa autonomous \\
  --platforms web \\
  --timeout 5m \\
  --quick-mode

if [ $? -ne 0 ]; then
  echo "QA issues found!"
  exit 1
fi

2. Nightly Regression

# CI Pipeline
schedule:
  - cron: "0 2 * * *"  # 2 AM daily

script:
  - ./helixqa autonomous \\
      --platforms android,web,desktop \\
      --timeout 4h \\
      --output nightly-results
  - ./scripts/upload-results.sh nightly-results

3. PR Validation

# Only test changed areas
./helixqa autonomous \\
  --diff-from main \\
  --smart-selection \\
  --timeout 15m

4. Production Monitoring

# Background crash detection
./helixqa monitor \\
  --production \\
  --alert-webhook $SLACK_WEBHOOK \\
  --continuous

Custom Integrations:

JIRA Ticket Creation:
```python
# scripts/jira-sync.py
import json
import requests

with open('qa-results/tickets/HQA-0001.md') as f:
    ticket = parse_ticket(f.read())

requests.post('https://jira.company.com/rest/api/2/issue',
    json={
        'fields': {
            'project': {'key': 'QA'},
            'summary': ticket.title,
            'description': ticket.description,
            'issuetype': {'name': 'Bug'},
            'priority': map_severity(ticket.severity)
        }
    })
```

Slack Notifications:
```bash
# scripts/notify-slack.sh
SUMMARY=$(jq -r '.summary' qa-results/report.json)
curl -X POST $SLACK_WEBHOOK \
  -H 'Content-type: application/json' \
  --data '{"text":"QA Complete: '$SUMMARY'"}'
```

Dashboard Integration:
```javascript
// Grafana panel
fetch('/api/qa-results/latest')
  .then(r => r.json())
  .then(data => {
    updateCoverageChart(data.coverage);
    updateIssueTable(data.issues);
  });
```

[Show integration architecture diagram]"
```

**[30:00-35:00] - Performance Optimization**
```
"Optimizing session performance:

1. Parallel Execution

GOMAXPROCS=8
HELIX_WORKER_POOL_SIZE=4

Distributes work across CPU cores
Reduces session duration by 60%

2. Smart Agent Selection

Strategy: Fastest viable model
- Quick checks: Groq (fast, cheap)
- Visual analysis: Claude (vision)
- Complex reasoning: GPT-4 (quality)

3. Caching

Enable response caching:
HELIX_CACHE_RESULTS=true
HELIX_CACHE_TTL=1h

Avoids redundant LLM calls
Reduces API costs by 40%

4. Incremental Testing

# Only test changed features
./helixqa autonomous \\
  --incremental \\
  --since-last-run

5. Resource Management

Memory limits:
HELIX_MAX_MEMORY_MB=2048

CPU limits:
HELIX_MAX_GOROUTINES=20

Video compression:
HELIX_VIDEO_QUALITY=medium  # vs high

Benchmarking:

Run performance test:
./helixqa benchmark \\
  --duration 1h \\
  --load 100-features

Results:
- Avg feature verification: 12s
- LLM calls per feature: 2.3
- API cost per session: $2.50
- Coverage rate: 94%

Optimization Checklist:

□ Use appropriate strategy
□ Enable caching
□ Limit platforms if not needed
□ Adjust video quality
□ Parallelize agents
□ Monitor API costs
□ Review timeout settings

In Module 5, we'll cover troubleshooting 
common issues that may arise."
```

---

## Module 5: Troubleshooting (20 minutes)

### Learning Objectives
- Diagnose common setup issues
- Debug session failures
- Optimize for specific scenarios
- Handle API rate limits

### Script

**[00:00-05:00] - Common Setup Issues**
```
"Let's troubleshoot the most common issues:

Issue 1: 'No agents available'

Symptoms:
- Session fails immediately
- Error: 'no suitable agent found'

Diagnosis:
which opencode  # Check if installed
which claude    # Check if installed

Solutions:
1. Install missing agents:
   go install github.com/opencode-ai/opencode@latest
   
2. Check .env paths:
   HELIX_AGENT_OPENCODE_PATH=/usr/local/bin/opencode
   
3. Verify API keys:
   echo $ANTHROPIC_API_KEY  # Should not be empty
   
4. Test agent manually:
   opencode --version

Issue 2: 'Vision provider failed'

Symptoms:
- Screenshots not analyzed
- Error: 'vision analysis failed'

Diagnosis:
- Check API key validity
- Verify model supports vision
- Check rate limits

Solutions:
1. Use vision-capable model:
   ANTHROPIC_MODEL=claude-3-opus-20240229
   
2. Fallback to non-vision:
   HELIX_VISION_PROVIDER=none
   
3. Check API quota:
   curl https://api.anthropic.com/v1/models \\
     -H "x-api-key: $KEY"

Issue 3: 'ADB device not found'

Symptoms:
- Android tests fail
- Error: 'device offline'

Solutions:
1. Check device connection:
   adb devices
   
2. Authorize device:
   adb kill-server
   adb start-server
   adb devices  # Accept RSA key
   
3. Update device ID in .env:
   HELIX_ANDROID_DEVICE=emulator-5554

[Show troubleshooting flowchart]"
```

**[05:00-12:00] - Session Debugging**
```
"When sessions fail or produce unexpected results:

Debug Mode:

./helixqa autonomous \\
  --verbose \\
  --debug \\
  --log-level debug

This shows:
- Every LLM prompt and response
- Navigation decisions
- Screenshot analysis details
- Timing information

Analyzing Logs:

# Filter for errors
grep "ERROR" qa-results/session.log

# Find slow operations
grep "duration" qa-results/session.log | sort -k2

# Track agent usage
grep "agent" qa-results/session.log | wc -l

Common Log Patterns:

Pattern: 'timeout waiting for response'
Cause: LLM API slow or rate limited
Fix: Increase HELIX_AGENT_TIMEOUT

Pattern: 'navigation failed after 3 retries'
Cause: UI changed or element not found
Fix: Update selectors or use vision

Pattern: 'screenshot capture failed'
Cause: Display not available or ffmpeg missing
Fix: Check display, install ffmpeg

Interactive Debugging:

Pause session at specific point:

./helixqa autonomous \\
  --breakpoint "after-login" \\
  --interactive

In interactive mode:
- Inspect current state
- Manually trigger actions
- Review screenshots
- Continue or abort

Session Replay:

./helixqa replay \\
  --session-id abc123 \\
  --from-timeline \\
  --speed 2x

Replays actions from timeline
Useful for reproducing issues"
```

**[12:00-18:00] - API Rate Limits & Costs**
```
"Managing API usage and costs:

Rate Limit Handling:

When you see:
'429 Too Many Requests'

HelixQA automatically:
1. Backs off exponentially
2. Retries with jitter
3. Switches to fallback provider
4. Logs rate limit events

Manual Rate Limit Configuration:

ANTHROPIC_RATE_LIMIT=40  # requests per minute
ANTHROPIC_RETRY_MAX=5
ANTHROPIC_RETRY_DELAY=2s

Cost Optimization:

Monitor costs:
./helixqa costs --session-id abc123

Output:
Provider      Tokens    Cost
Anthropic     45,230    $2.34
OpenAI        12,100    $0.89
Total                   $3.23

Cost Reduction Strategies:

1. Use cheaper models for simple tasks:
   - GPT-3.5 for text analysis
   - Claude Haiku for quick checks
   
2. Enable caching:
   HELIX_CACHE_RESULTS=true
   Cache hit rate: ~40%
   
3. Reduce screenshot frequency:
   HELIX_SCREENSHOT_INTERVAL=5s  # vs 1s
   
4. Shorter timeouts:
   HELIX_AGENT_TIMEOUT=30s  # vs 120s
   
5. Use vision selectively:
   HELIX_VISION_ENABLED=phases=curiosity-only

Budget Alerts:

Set spending limit:
HELIX_MAX_COST_PER_SESSION=5.00

Alert at 80%:
HELIX_COST_ALERT_THRESHOLD=0.8

Webhook notification:
HELIX_COST_ALERT_WEBHOOK=https://hooks.slack.com/...

Multi-Provider Load Balancing:

Distribute load across providers:

Provider      Weight    Use Case
Anthropic     50%       Vision tasks
OpenAI        30%       Text analysis
Google        20%       Fallback

Automatic failover when:
- Rate limit hit
- Service unavailable
- Cost threshold reached

[Show cost monitoring dashboard]"
```

**[18:00-20:00] - Getting Help & Resources**
```
"When you're stuck:

Documentation:
- README.md - Quick start
- docs/USER_GUIDE.md - Detailed usage
- docs/ARCHITECTURE_DIAGRAMS.md - System design
- docs/TROUBLESHOOTING.md - Common issues

Community Resources:
- GitHub Issues: github.com/vasic-digital/Catalogizer/issues
- Discussions: GitHub Discussions tab
- Wiki: Project wiki with examples

Debug Information to Collect:

When reporting issues, include:
1. Session logs: qa-results/session.log
2. Configuration: .env (redact API keys)
3. System info: OS, Go version
4. Command used: Full helixqa command
5. Expected vs actual behavior

Example Report:

Title: Session timeout on Android testing

Description:
Running autonomous session on Android emulator.
Session times out after 10 minutes with 20% coverage.

Environment:
- OS: Ubuntu 22.04
- Go: 1.24
- HelixQA: v1.0.0

Command:
./helixqa autonomous --platforms android --timeout 30m

Logs:
[ERROR] adb shell timeout
[ERROR] screenshot capture failed

Expected: Complete testing in 30m
Actual: Timeout at 10m

Next Steps:

Now that you've completed this course, you can:

1. Set up HelixQA for your project
2. Configure custom strategies
3. Integrate with CI/CD
4. Monitor and optimize performance
5. Contribute to the project

Thank you for watching!

For updates and advanced topics, 
subscribe to the channel and star the repo.

Happy autonomous testing!"
```

---

## Module 6: Storage and Media Management (20 minutes)

### Learning Objectives
- Configure storage roots across all 5 protocols
- Understand the media scanning and detection pipeline
- Browse and search media entities
- Manage media metadata and cover art

### Prerequisites
- Catalogizer backend running
- At least one accessible media source (local directory, SMB share, etc.)

### Script

```
[0:00-2:00] Introduction
"In this module, we configure storage roots and scan media collections.
Catalogizer supports 5 protocols: local filesystem, SMB/CIFS, FTP, NFS, and WebDAV."

[2:00-5:00] Adding Storage Roots
"Navigate to Admin > Storage Roots. Click Add Root.
For a local path: enter the directory path, e.g., /media/movies.
For an SMB share: enter the server address, share name, and credentials.

# Via API:
curl -X POST http://localhost:8080/api/v1/storage/roots \
  -H 'Authorization: Bearer TOKEN' \
  -d '{"path": "/media/movies", "name": "Movies", "protocol": "local"}'

Test the connection before saving."

[5:00-9:00] Scanning Media
"Once a root is added, click Scan to start detection.
The pipeline works in 3 stages:
1. File Detection — identifies media files by extension and content
2. Media Analysis — parses titles, extracts metadata (year, quality, codec)
3. Aggregation — creates entities, builds hierarchy, detects duplicates

Progress appears in the UI and via WebSocket events.
Catalogizer recognizes 11 media types:
movie, tv_show, tv_season, tv_episode, music_artist, music_album,
song, game, software, book, comic."

[9:00-14:00] Browsing and Searching
"Open the Entity Browser to see all detected media.
Use the search bar for text search.
Filter by type, year, rating, or quality.
Click any entity to see details: files, metadata, hierarchy.

For TV shows, you'll see the hierarchy:
Show > Seasons > Episodes
For music: Artist > Albums > Songs"

[14:00-18:00] Metadata and Cover Art
"Catalogizer enriches entities with metadata from external providers:
- TMDB and OMDB for movies and TV
- MusicBrainz for music
- OpenLibrary for books

Cover art is automatically fetched and cached.
You can also upload custom cover art via the entity detail page."

[18:00-20:00] Summary
"You've learned to add storage roots, scan media, browse entities,
and understand the metadata enrichment pipeline.
Next: Module 7 covers collections and playback."
```

---

## Module 7: Collections and Playback (20 minutes)

### Learning Objectives
- Create and manage collections (manual and smart)
- Use the playlist system
- Play media with subtitle support
- Import and export collections

### Prerequisites
- Scanned media library with entities
- Web browser or desktop app

### Script

```
[0:00-2:00] Introduction
"This module covers organizing media into collections and playing content.
Collections let you group media by any criteria — manually or with smart rules."

[2:00-6:00] Creating Collections
"Navigate to Collections > New Collection.
Enter a name and optional description.
For manual collections, add items by searching and clicking Add.

For smart collections, enable 'Smart Collection' and define rules:
- media_type equals 'movie'
- year greater_than 2020
- rating greater_than 7.0

Smart collections automatically update when new matching media is scanned."

[6:00-10:00] Import and Export
"Collections can be exported as JSON or CSV:
Click Export > Choose format > Download.

Import works the same way:
Collections > Import > Upload JSON/CSV file.
M3U playlists are also supported for music collections."

[10:00-14:00] Playlist Management
"Playlists are ordered sequences for playback.
Create a playlist: Playlists > New Playlist.
Add items by drag-and-drop or search.
Reorder by dragging items up and down.

# Via API:
curl -X POST http://localhost:8080/api/v1/playlists \
  -H 'Authorization: Bearer TOKEN' \
  -d '{"name": "Movie Night", "items": [1, 5, 12]}'
"

[14:00-18:00] Media Playback
"Click any media item to open the player.
The player supports:
- Play/pause, seek, volume
- Fullscreen mode
- Subtitle selection and sync

Upload subtitles via the Subtitle tab:
Click Upload > Select .srt or .vtt file.
Use the Sync tool to adjust timing if needed."

[18:00-20:00] Summary
"You can now organize media into collections, create playlists,
play content with subtitles, and import/export your library.
Next: Module 8 covers the Android mobile app."
```

---

## Module 8: Android Mobile App (15 minutes)

### Learning Objectives
- Install and configure the Catalogizer Android app
- Connect the app to a running catalog-api server
- Browse, search, and play media from your phone
- Configure offline mode and background sync
- Customize app settings for your workflow

### Prerequisites
- Android device running Android 8.0 (API 26) or later
- Running catalog-api server accessible on the local network
- Admin or user account credentials
- APK file downloaded or access to the release artifacts

### Script

**[00:00-02:00] - Introduction & APK Installation**
```
"Welcome to Module 8! In this video, we'll set up the
Catalogizer Android app on your phone or tablet.

First, let's install the APK. You have two options:

Option 1: Direct APK install
- Transfer the APK to your device via USB or file share
- Open the APK file
- Allow installation from unknown sources if prompted
- Tap Install

Option 2: ADB install from your computer
```bash
adb install catalogizer-android-v2.1.0.apk
```

If you see 'Success' in the terminal, you're good to go.

Once installed, find the Catalogizer icon in your app drawer
and tap to launch it."
```

**[02:00-04:00] - Server Discovery & Login**
```
"When you first launch the app, you'll see the server
connection screen.

The app uses mDNS to automatically discover catalog-api
servers on your local network. If your server is running,
it should appear in the list within a few seconds.

If automatic discovery doesn't find your server:
1. Tap 'Manual Configuration'
2. Enter the server URL (e.g., http://192.168.0.100:8080)
3. Tap 'Test Connection' to verify

Once connected, you'll see the login screen.
Enter your username and password, then tap 'Sign In'.

The app stores your session token securely using
Android Keystore, so you won't need to log in
every time you open the app."
```

**[04:00-07:00] - Browsing Media**
```
"After login, you're on the home screen.

The bottom navigation bar has five tabs:
- Home: Dashboard with recent items and recommendations
- Browse: Full media library organized by type
- Search: Global search across all media
- Downloads: Offline content
- Profile: Settings and account

Let's explore the Browse tab. You'll see categories:
- Movies, TV Shows, Music, Games, Books, and more

Tap any category to see items. Each item shows:
- Cover art or poster
- Title and year
- Rating (if available)
- Quality indicators (4K, HDR)

Tap an item to see its detail page:
- Full metadata (cast, crew, description)
- Available files with quality info
- Play button for immediate streaming
- Download button for offline access
- Add to collection or favorites

[Show browsing through movie categories]

Pull down to refresh any list.
Long-press an item for quick actions."
```

**[07:00-09:00] - Search & Filtering**
```
"The Search tab is powerful. Tap the search bar and
start typing.

Search works across:
- Titles (movies, shows, albums, etc.)
- People (actors, directors, artists)
- Descriptions and metadata

Results update as you type with debounced requests.

Advanced filtering:
- Tap the filter icon next to the search bar
- Filter by media type, year range, rating
- Sort by name, date added, rating, or year
- Combine multiple filters

Recent searches are saved for quick access.
You can also use voice search by tapping the
microphone icon."
```

**[09:00-11:30] - Offline Mode & Background Sync**
```
"One of the best features of the Android app is
offline support.

To download content for offline use:
1. Open any media item
2. Tap the download icon
3. Select quality (Original, High, Medium, Low)
4. Download starts in the background

Check download progress in the Downloads tab.
A notification also shows progress.

Background sync keeps your library up to date:
- New items from scans appear automatically
- Watch progress syncs across all devices
- Metadata updates pull in the background

Configure sync in Settings > Sync:
- Sync frequency (15 min, 30 min, 1 hour, manual)
- Wi-Fi only (recommended to save mobile data)
- Auto-download favorites
- Storage limit for offline content

When you're offline:
- Downloaded content plays normally
- Browse history is cached
- Changes queue and sync when back online

[Show downloading a movie and playing it offline]"
```

**[11:30-13:30] - Playback**
```
"Tap Play on any media item to start streaming.

The built-in player supports:
- Video: Hardware-accelerated playback
- Audio track selection
- Subtitle support (SRT, ASS, VTT)
- Playback speed control (0.5x to 2x)
- Resume from last position
- Picture-in-picture mode
- Chromecast support

For music, the player shows:
- Album art
- Track info
- Mini player in bottom bar
- Full-screen mode with visualizations
- Queue management

Casting to other devices:
- Tap the cast icon in the player
- Select your Chromecast or smart TV
- Control playback from your phone"
```

**[13:30-15:00] - Settings & Summary**
```
"Let's review the key settings under Profile > Settings:

Server: Change or add server connections
Sync: Background sync preferences
Downloads: Storage management, quality defaults
Playback: Default quality, subtitle preferences
Appearance: Theme (light/dark/system), language
Notifications: Scan alerts, new content alerts
Storage: Clear cache, manage downloads

Tips for best experience:
- Keep the app updated for latest features
- Use Wi-Fi for initial library sync
- Enable background sync for seamless updates
- Set download quality based on your storage

That's the Android app! In Module 9, we'll explore
the Android TV experience with D-pad remote navigation."
```

---

## Module 9: Android TV App (12 minutes)

### Learning Objectives
- Install Catalogizer on an Android TV device via ADB
- Navigate the app using a D-pad remote control
- Browse and play media on the big screen
- Use voice search and home screen channels
- Understand Android TV input handling quirks

### Prerequisites
- Android TV device (Xiaomi Mi Box, NVIDIA Shield, or similar)
- ADB access enabled on the Android TV device
- Running catalog-api server on the local network
- Computer with ADB tools for initial installation

### Script

**[00:00-02:30] - Installation via ADB**
```
"Welcome to Module 9! Let's set up Catalogizer on
your Android TV device.

Android TV apps are installed via ADB since we're
not distributing through the Play Store yet.

Step 1: Enable Developer Options on your TV
- Settings > About > Build Number (tap 7 times)
- Settings > Developer Options > USB Debugging: ON
- Note: Also enable 'Network Debugging' for wireless ADB

Step 2: Connect via ADB
```bash
# Find your TV's IP address (Settings > Network)
adb connect 192.168.0.214:5555

# Verify connection
adb devices
# Should show: 192.168.0.214:5555  device
```

Step 3: Install the APK
```bash
adb install catalogizer-androidtv-v2.1.0.apk
```

Step 4: Set up ADB reverse proxy so the TV can
reach your catalog-api server:
```bash
adb reverse tcp:8080 tcp:8080
```

This maps localhost:8080 on the TV to your server.

Find Catalogizer in your TV's app drawer and launch it."
```

**[02:30-05:00] - D-pad Navigation & Login**
```
"Android TV is entirely D-pad driven. No touchscreen.
Understanding D-pad navigation is critical.

The remote has these key inputs:
- D-pad: Up, Down, Left, Right
- Center/Select: Confirm action (dpad_center)
- Back: Go back one screen
- Home: Return to Android TV home

CRITICAL NOTE FOR DEVELOPERS:
When entering text on Android TV, you must send
dpad_center BEFORE type commands, and use
KEYCODE_TAB to move between input fields.
This is an Android TV platform requirement that
differs from standard Android.

Login flow on Android TV:
1. The app opens to the server connection screen
2. Use D-pad to navigate to the server URL field
3. Press Center to activate the field
4. Use the on-screen keyboard to enter the URL
   (or it auto-discovers via mDNS)
5. Press KEYCODE_TAB to move to username field
6. Press Center, then type your username
7. Tab to password, Center, type password
8. Navigate to Sign In button and press Center

[Show the complete login flow on Mi Box]

Once logged in, you'll see the home screen with
content rails - rows of media organized by category."
```

**[05:00-07:30] - Browsing Media on the Big Screen**
```
"The Android TV interface uses a 'lean-back' design
optimized for 10-foot viewing.

Home Screen Layout:
- Top row: Search and settings
- Featured rail: Highlighted content with large cards
- Category rails: Movies, TV Shows, Music, etc.
- Continue Watching rail: Resume where you left off
- Recently Added rail: New content from latest scans

Navigation:
- Left/Right: Scroll within a rail
- Up/Down: Move between rails
- Center: Select an item
- Long press: Quick actions menu

Each media card shows:
- Poster/cover art (landscape format for TV)
- Title overlay
- Quality badges (4K, HDR)

Select an item to see its detail screen:
- Large backdrop image
- Full metadata display
- Play, Add to List, More Info buttons
- Related items rail at the bottom

The interface responds to focus changes with
smooth animations and scaling effects.

[Show browsing through movie and TV show rails]"
```

**[07:30-09:30] - Playback on the Big Screen**
```
"Press Play on any item to start playback.

The TV player is optimized for the big screen:
- Full-screen playback with no UI chrome
- Press Center to show/hide playback controls
- Left/Right: Seek backward/forward (10s increments)
- Up: Show chapter or episode selector
- Down: Show audio and subtitle options

Playback controls overlay:
- Play/Pause
- Progress bar with thumbnail preview
- Time elapsed / total duration
- Audio track selector
- Subtitle selector
- Quality selector (if transcoding available)

For TV shows:
- 'Next Episode' prompt at end of episode
- Auto-play next episode after 10 seconds
- Season selector accessible from Up button

For music:
- Album art displayed full screen
- Track info and progress
- Queue visible with Up button
- Shuffle and repeat toggles

[Show playing a movie with subtitle selection]"
```

**[09:30-11:00] - Voice Search & Home Screen Channels**
```
"If your remote has a microphone button, you can
use voice search.

Press the microphone button and say:
- 'Play The Matrix'
- 'Show action movies'
- 'Search for Beatles'

The app registers as a voice search provider,
so results from your Catalogizer library appear
alongside other Android TV results.

Home Screen Channels:
Catalogizer adds channels to your Android TV home:
- Continue Watching: Resume your content
- Recommended: Based on your viewing history
- Recently Added: Latest items from scans

These channels appear on your TV's main home screen
even before you open the Catalogizer app.

To manage channels:
1. Long-press the channel row on home screen
2. Select 'Customize channel'
3. Choose which channels to show or hide"
```

**[11:00-12:00] - Tips & Summary**
```
"Tips for the best Android TV experience:

1. Use Ethernet instead of Wi-Fi for streaming
   - More stable for high-bitrate content
   - Lower latency for remote navigation

2. ADB reverse proxy must be re-established
   after TV reboots:
   adb connect 192.168.0.214:5555
   adb reverse tcp:8080 tcp:8080

3. For 4K content, ensure your TV supports
   the codec (H.265/HEVC recommended)

4. Clear app cache periodically:
   Settings > Apps > Catalogizer > Clear Cache

That wraps up the Android TV module!
In Module 10, we'll look at the Tauri desktop
application for Windows, macOS, and Linux."
```

---

## Module 10: Desktop App (10 minutes)

### Learning Objectives
- Install the Catalogizer desktop application
- Complete the installation wizard
- Connect to a catalog-api server
- Browse and play media from the desktop
- Use system tray integration and local scanning

### Prerequisites
- Windows 10+, macOS 12+, or Linux with X11/Wayland
- Downloaded desktop installer or AppImage
- Running catalog-api server (local or remote)

### Script

**[00:00-02:00] - Installation**
```
"Welcome to Module 10! Let's install the Catalogizer
desktop application built with Tauri.

The desktop app is available for all major platforms:

Windows:
- Download Catalogizer-Setup.exe
- Run the installer
- Follow the wizard (next, next, finish)

macOS:
- Download Catalogizer.dmg
- Drag to Applications folder
- First launch: Right-click > Open (Gatekeeper)

Linux:
- AppImage: chmod +x Catalogizer.AppImage && ./Catalogizer.AppImage
- Or use the .deb package: sudo dpkg -i catalogizer.deb

The app is lightweight - the Tauri runtime is much
smaller than Electron-based alternatives.

Launch the application to start the setup wizard."
```

**[02:00-04:00] - Installation Wizard**
```
"The installation wizard walks you through initial
configuration in four steps.

Step 1: Server Connection
- Enter your catalog-api server URL
- Or select a discovered server from the list
- Test connection to verify

Step 2: Authentication
- Enter your username and password
- Option to 'Remember me' for auto-login
- Server validates credentials immediately

Step 3: Local Storage (Optional)
- Point to local media folders
- These can be scanned independently
- Useful if you have media on your desktop machine

Step 4: Preferences
- Theme: Light, Dark, or System
- Default media player: Built-in or external
- Notification preferences
- Startup behavior (launch at login, minimize to tray)

Click 'Finish' and the main interface loads.

[Show the complete wizard flow]"
```

**[04:00-06:30] - Main Interface & Browsing**
```
"The desktop interface mirrors the web UI but with
native OS integration.

Layout:
- Title bar with native window controls
- Left sidebar: Navigation (Browse, Search, Collections)
- Main area: Content grid or list view
- Bottom bar: Now playing / mini player

Browsing works the same as the web interface:
- Click media type in sidebar to filter
- Grid view shows posters with hover details
- List view shows more metadata per item
- Sort and filter controls at top

Desktop-specific features:
- Drag and drop files to add to library
- Right-click context menus on items
- Keyboard shortcuts for power users:
  - Ctrl+F: Search
  - Ctrl+L: Library view
  - Space: Play/Pause
  - F11: Fullscreen

Double-click any item to open its detail view.
Click Play to start streaming through the built-in
player or your configured external player.

[Show browsing and opening a movie detail page]"
```

**[06:30-08:30] - System Tray & Local Scanning**
```
"The desktop app integrates with your system tray.

System Tray Features:
- Minimize to tray (keeps running in background)
- Quick access menu:
  - Open Catalogizer
  - Recently Played
  - Start Scan
  - Settings
  - Quit

The tray icon shows notifications for:
- Scan completion
- New content added
- Server connection changes

Local Scanning:
The desktop app can scan folders on your machine
directly, without going through the server.

To set up local scanning:
1. Go to Settings > Local Storage
2. Add folder paths to monitor
3. Enable 'Watch for changes' for real-time detection
4. Set scan schedule (or manual only)

Scanned files are registered with the catalog-api
server and become available on all your devices.

This is especially useful if you download media
to your desktop and want it cataloged automatically."
```

**[08:30-10:00] - Playback & Summary**
```
"The desktop player uses native rendering for
optimal performance.

Player features:
- Hardware-accelerated video decode
- Subtitle rendering (SRT, ASS, VTT)
- Audio track selection
- Keyboard controls:
  - Space: Play/Pause
  - Left/Right: Seek 10 seconds
  - Up/Down: Volume
  - F: Fullscreen
  - M: Mute
  - S: Cycle subtitles

The player remembers your position across sessions
and syncs with the server, so you can continue
watching on any device.

Summary:
- Lightweight native app built with Tauri
- Full browsing, search, and playback
- System tray for background operation
- Local folder scanning and monitoring
- Cross-device sync via catalog-api

In Module 11, we'll explore monitoring and
observability for your Catalogizer deployment."
```

---

## Module 11: Monitoring and Observability (15 minutes)

### Learning Objectives
- Set up Prometheus metrics collection for catalog-api
- Build Grafana dashboards for key metrics
- Configure alerting for critical conditions
- Interpret metrics to diagnose performance issues
- Use health checks and log analysis for troubleshooting

### Prerequisites
- Running catalog-api instance
- Podman or Docker for running Prometheus and Grafana containers
- Basic understanding of time-series metrics

### Script

**[00:00-03:00] - Observability Overview & Prometheus Setup**
```
"Welcome to Module 11! Monitoring is essential for
any production deployment.

Catalogizer exposes Prometheus metrics on the
/metrics endpoint. Let's set up the monitoring stack.

First, catalog-api must be running. Verify the
metrics endpoint:
```bash
curl http://localhost:8080/metrics
```

You should see metrics in Prometheus exposition format.

Now let's start Prometheus:
```bash
podman run -d --name prometheus \
  --network host \
  -v $(pwd)/monitoring/prometheus.yml:/etc/prometheus/prometheus.yml \
  docker.io/prom/prometheus:latest
```

The prometheus.yml scrape config:
```yaml
scrape_configs:
  - job_name: 'catalogizer-api'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:8080']
```

Open http://localhost:9090 to access Prometheus UI.

Verify the target is up:
Status > Targets > catalogizer-api should show 'UP'."
```

**[03:00-06:00] - Key Metrics & Grafana Dashboards**
```
"Let's explore the metrics Catalogizer exposes:

HTTP Metrics:
- http_requests_total: Request count by method, path, status
- http_request_duration_seconds: Latency histogram
- http_requests_in_flight: Active requests

Business Metrics:
- catalogizer_scans_total: Completed scans
- catalogizer_files_detected: Files found per scan
- catalogizer_media_items_total: Total media entities
- catalogizer_websocket_connections: Active WS clients

System Metrics:
- go_goroutines: Active goroutines
- go_memstats_alloc_bytes: Memory usage
- process_cpu_seconds_total: CPU time

Now let's visualize with Grafana:
```bash
podman run -d --name grafana \
  --network host \
  docker.io/grafana/grafana:latest
```

Open http://localhost:3001 (default: admin/admin).

Add Prometheus as a data source:
1. Configuration > Data Sources > Add
2. Select Prometheus
3. URL: http://localhost:9090
4. Save & Test

Import the Catalogizer dashboard:
1. Dashboards > Import
2. Upload monitoring/grafana-dashboard.json
3. Select Prometheus data source

[Show the dashboard with live metrics]

Key panels to watch:
- Request rate (requests/sec over time)
- Latency percentiles (p50, p95, p99)
- Error rate (4xx and 5xx responses)
- Active connections (HTTP + WebSocket)
- Memory and goroutine trends"
```

**[06:00-09:00] - Alert Configuration**
```
"Alerts notify you before problems become outages.

In Grafana, set up alert rules:

Alert 1: High Error Rate
- Condition: rate(http_requests_total{status=~'5..'}[5m]) > 0.1
- Threshold: More than 10% of requests are errors
- Duration: For 5 minutes
- Severity: Critical

Alert 2: High Latency
- Condition: histogram_quantile(0.95, http_request_duration_seconds) > 2
- Threshold: p95 latency exceeds 2 seconds
- Duration: For 10 minutes
- Severity: Warning

Alert 3: Memory Leak Detection
- Condition: go_memstats_alloc_bytes > 2e9
- Threshold: Memory exceeds 2 GB
- Duration: For 15 minutes
- Severity: Warning

Alert 4: Goroutine Leak
- Condition: go_goroutines > 1000
- Threshold: More than 1000 goroutines
- Duration: For 5 minutes
- Severity: Critical

Configure notification channels:
1. Alerting > Contact Points
2. Add email, Slack, or webhook
3. Assign contact points to alert rules

[Show creating an alert rule step by step]

Prometheus alerting rules can also be defined in
monitoring/alert-rules.yml for infrastructure-level
alerts independent of Grafana."
```

**[09:00-12:00] - Health Checks & Log Analysis**
```
"Health checks provide quick status verification.

The API exposes several health endpoints:

```bash
# Basic health check
curl http://localhost:8080/api/v1/health
# Returns: {'status': 'ok', 'uptime': '2h15m'}

# Detailed health with dependency status
curl http://localhost:8080/api/v1/health/detailed
# Returns: database, cache, storage status
```

Use health checks for:
- Load balancer probes (every 10 seconds)
- Container orchestration liveness/readiness
- Uptime monitoring services

Log Analysis:

catalog-api logs structured JSON to stdout:
```json
{
  'level': 'info',
  'time': '2026-04-03T10:15:00Z',
  'method': 'GET',
  'path': '/api/v1/media',
  'status': 200,
  'latency': '45ms'
}
```

Useful log queries:

# Find slow requests (>1 second)
```bash
podman logs catalog-api 2>&1 | \
  jq 'select(.latency_ms > 1000)'
```

# Find errors
```bash
podman logs catalog-api 2>&1 | \
  jq 'select(.level == \"error\")'
```

# Count requests per endpoint
```bash
podman logs catalog-api 2>&1 | \
  jq -r '.path' | sort | uniq -c | sort -rn
```

For centralized logging, forward to a log
aggregation service like Loki or Elasticsearch."
```

**[12:00-15:00] - Interpreting Metrics & Summary**
```
"Let's look at common patterns and what they mean:

Pattern: Latency spikes during scans
- Normal behavior: Scanning is I/O intensive
- Action: Schedule scans during off-peak hours

Pattern: Memory steadily increasing
- Possible leak: Check goroutine count
- Action: Monitor and restart if needed
- Catalog-api uses Memory module for leak detection

Pattern: WebSocket connections not dropping
- Possible issue: Clients not disconnecting cleanly
- Action: Check WebSocket handler cleanup

Pattern: High 429 (rate limit) responses
- Clients hitting rate limits
- Action: Review rate limiter configuration
- Check if legitimate or potential abuse

Dashboard Best Practices:
1. Keep dashboards focused (one per concern)
2. Set appropriate time ranges (last 1h for ops, 7d for trends)
3. Use template variables for filtering
4. Document panel descriptions
5. Review dashboards in team meetings

Resource limits reminder:
When running monitoring containers, respect the host
resource limits (30-40% max):
```bash
podman run --cpus=0.5 --memory=512m prometheus
podman run --cpus=0.5 --memory=512m grafana
```

That covers monitoring! In Module 12, we'll dive
into security hardening for your deployment."
```

---

## Module 12: Security and Hardening (12 minutes)

### Learning Objectives
- Configure HTTPS and HTTP/3 (QUIC) for secure transport
- Set up JWT authentication with proper secret management
- Implement role-based access control
- Configure rate limiting and CORS
- Run security scanning tools (SonarQube, Snyk, Semgrep)

### Prerequisites
- Running catalog-api instance
- Basic understanding of TLS/SSL
- Podman for running security scanning tools

### Script

**[00:00-02:30] - HTTPS & HTTP/3 Setup**
```
"Welcome to Module 12! Security is not optional.
Let's harden your Catalogizer deployment.

Catalogizer uses HTTP/3 (QUIC) by default, which
provides encrypted transport with lower latency
than traditional HTTPS.

On startup, catalog-api generates self-signed TLS
certificates automatically. For production, use
proper certificates:

1. Obtain certificates (Let's Encrypt recommended):
```bash
certbot certonly --standalone -d catalogizer.yourdomain.com
```

2. Configure in .env:
```
TLS_CERT_PATH=/etc/letsencrypt/live/catalogizer.yourdomain.com/fullchain.pem
TLS_KEY_PATH=/etc/letsencrypt/live/catalogizer.yourdomain.com/privkey.pem
```

3. The server starts with HTTP/3 + Brotli compression.
   Fallback chain: HTTP/3 (QUIC) > HTTP/2 + gzip > HTTP/1.1
   Production deployments must never use plain HTTP/1.1.

For the Nginx reverse proxy (production setup):
```nginx
server {
    listen 443 ssl http2;
    listen 443 quic reuseport;

    ssl_certificate /path/to/fullchain.pem;
    ssl_certificate_key /path/to/privkey.pem;

    add_header Alt-Svc 'h3=\":443\"; ma=86400';

    location /api/ {
        proxy_pass http://localhost:8080;
    }
}
```

[Show verifying HTTP/3 with curl]
```bash
curl --http3 https://catalogizer.yourdomain.com/api/v1/health
```"
```

**[02:30-05:00] - JWT Authentication & Roles**
```
"Catalogizer uses JWT tokens for authentication.

Configure JWT in .env:
```
JWT_SECRET=your-very-long-random-secret-at-least-32-chars
JWT_EXPIRY=24h
JWT_REFRESH_EXPIRY=7d
```

CRITICAL: Never use default secrets in production.
Generate a strong secret:
```bash
openssl rand -base64 64
```

Role-Based Access Control (RBAC):

Catalogizer defines three roles:
- admin: Full access (user management, settings, scanning)
- user: Browse, play, create collections, download
- guest: Browse and play only (read-only)

Assign roles via the admin panel or API:
```bash
curl -X PUT http://localhost:8080/api/v1/users/5/role \
  -H 'Authorization: Bearer <admin-token>' \
  -H 'Content-Type: application/json' \
  -d '{\"role\": \"user\"}'
```

Token refresh flow:
1. Client sends request with expired access token
2. Server returns 401
3. Client uses refresh token to get new access token
4. Original request retries automatically

The API client library handles this transparently."
```

**[05:00-07:30] - Rate Limiting & CORS**
```
"Rate limiting protects against abuse and DoS attacks.

Catalogizer has two rate limit tiers:

Strict (login/register endpoints):
- 5 requests per minute per IP
- Prevents brute-force password attacks

Default (all other endpoints):
- 100 requests per minute per IP
- Sufficient for normal usage

Configure in .env:
```
RATE_LIMIT_DEFAULT=100
RATE_LIMIT_STRICT=5
RATE_LIMIT_WINDOW=60s
```

When rate limited, the API returns:
- HTTP 429 Too Many Requests
- Retry-After header with wait time
- X-RateLimit-Remaining header

CORS Configuration:

Cross-Origin Resource Sharing must be configured
for your frontend domain:

```
CORS_ALLOWED_ORIGINS=https://catalogizer.yourdomain.com
CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=Authorization,Content-Type
CORS_MAX_AGE=3600
```

For development, you can allow all origins,
but NEVER do this in production:
```
CORS_ALLOWED_ORIGINS=*  # Development only!
```

[Show testing rate limiting with rapid requests]"
```

**[07:30-10:00] - Security Scanning**
```
"Regular security scanning catches vulnerabilities
before they become incidents.

Catalogizer includes several scanning tools:

1. Go Vulnerability Check:
```bash
cd catalog-api
govulncheck ./...
```
Checks Go dependencies against the vulnerability database.

2. npm Audit (frontend):
```bash
cd catalog-web
npm audit
```
Reports known vulnerabilities in npm packages.

3. Semgrep (static analysis):
```bash
podman-compose -f docker-compose.security.yml \
  --profile semgrep-scan run --rm semgrep-scanner
```
Finds security anti-patterns in source code.

4. SonarQube (code quality + security):
```bash
./scripts/run-sonarqube-scan.sh
```
Comprehensive analysis including:
- Security hotspots
- Code smells
- Bug detection
- Duplicate code

5. Snyk and Trivy (container scanning):
```bash
podman-compose -f docker-compose.security.yml \
  --profile snyk run --rm snyk-scanner
```

Run all scans together:
```bash
./scripts/security-scan.sh
```

[Show running govulncheck and reviewing results]

Make scanning part of your regular workflow:
- Before every release
- Weekly automated scans
- After dependency updates"
```

**[10:00-12:00] - Security Checklist & Summary**
```
"Production Security Checklist:

Transport:
[ ] HTTPS/HTTP3 with valid certificates
[ ] Brotli compression enabled
[ ] HSTS header configured
[ ] No plain HTTP in production

Authentication:
[ ] Strong JWT secret (64+ characters)
[ ] Token expiry configured (24h max)
[ ] Refresh tokens enabled
[ ] Admin password changed from default

Authorization:
[ ] RBAC roles assigned to all users
[ ] Guest access restricted appropriately
[ ] API endpoints require authentication
[ ] Admin endpoints require admin role

Rate Limiting:
[ ] Strict rate limit on auth endpoints
[ ] Default rate limit on API endpoints
[ ] Rate limit headers in responses

Network:
[ ] CORS restricted to known domains
[ ] Firewall rules for API port
[ ] Database not exposed to public network
[ ] Redis protected with password

Scanning:
[ ] govulncheck: 0 vulnerabilities
[ ] npm audit: 0 critical/production vulnerabilities
[ ] Semgrep: 0 high-severity findings
[ ] Container images scanned

Secrets:
[ ] .env files not in git
[ ] .gitignore covers all .env files
[ ] API keys rotated regularly
[ ] No secrets in source code

That's security hardening! In Module 13, we'll
explore the Challenge System for automated testing."
```

---

## Module 13: Challenge System (10 minutes)

### Learning Objectives
- Understand what challenges are and how they verify system correctness
- Run individual challenges and the full suite
- Interpret challenge results and fix failures
- Explore the challenge bank and custom challenge creation
- Use challenge-based QA as part of your workflow

### Prerequisites
- Running catalog-api server with at least one storage root configured
- Basic understanding of REST APIs
- Access to the admin account

### Script

**[00:00-02:30] - What Are Challenges?**
```
"Welcome to Module 13! The Challenge System is
Catalogizer's built-in verification framework.

Think of challenges as structured, automated tests
that verify your entire deployment works correctly -
from API endpoints to data integrity.

There are currently 492 registered challenges:
- 50 original system challenges (CH-001 to CH-050)
- 174 user flow challenges (UF-*)
- 15 module verification challenges (MOD-*)
- 253 HelixQA test bank challenges

Challenges are Go structs that embed BaseChallenge
and implement an Execute() method. They're registered
in catalog-api/challenges/register.go.

Categories include:
- API health and endpoint verification
- Data integrity (scanning, detection, entities)
- Authentication and authorization
- Performance (response times, throughput)
- Cross-platform user flows
- Module integration verification

The challenge system is exposed via REST endpoints:
- GET /api/v1/challenges - List all challenges
- POST /api/v1/challenges/:id/run - Run one
- POST /api/v1/challenges/run-all - Run all
- GET /api/v1/challenges/:id/results - Get results"
```

**[02:30-05:00] - Running Challenges**
```
"Let's run some challenges. First, ensure catalog-api
is running and you're authenticated.

Run a single challenge:
```bash
curl -X POST http://localhost:8080/api/v1/challenges/CH-001/run \
  -H 'Authorization: Bearer <your-token>'
```

Response:
```json
{
  'id': 'CH-001',
  'name': 'API Health Check',
  'status': 'passed',
  'duration': '45ms',
  'assertions': 5,
  'passed': 5,
  'failed': 0
}
```

Run all challenges (WARNING: this is synchronous
and blocks until complete - can take 25+ minutes
if NAS scanning is involved):
```bash
curl -X POST http://localhost:8080/api/v1/challenges/run-all \
  -H 'Authorization: Bearer <your-token>'
```

IMPORTANT: RunAll is blocking. No other challenge
can execute until it finishes. The runner has a
5-minute stale threshold - if no progress is reported
for 5 minutes, the stuck challenge is terminated.

Monitor progress by polling:
```bash
curl http://localhost:8080/api/v1/challenges/status \
  -H 'Authorization: Bearer <your-token>'
```

[Show running CH-001 through CH-005 individually]"
```

**[05:00-07:30] - Interpreting Results**
```
"Challenge results tell you exactly what passed
and what needs attention.

Result statuses:
- passed: All assertions succeeded
- failed: One or more assertions failed
- stuck: No progress for 5 minutes (killed)
- timed_out: Exceeded timeout limit
- skipped: Prerequisites not met

Example failure output:
```json
{
  'id': 'CH-012',
  'name': 'Media Entity Aggregation',
  'status': 'failed',
  'assertions': 8,
  'passed': 6,
  'failed': 2,
  'failures': [
    'Expected media_items count > 0, got 0',
    'Expected parent_id set for TV episodes'
  ]
}
```

Common failure patterns and fixes:

'count > 0, got 0':
- Run a scan first to populate the database
- Check storage root configuration

'connection refused':
- Verify catalog-api is running
- Check the port configuration

'unauthorized':
- Token expired - re-authenticate
- Check user role permissions

'timeout':
- Increase config.json write_timeout to 900
- Check if NAS is accessible

The challenge bank in challenges/config/ defines
additional test parameters and expected values."
```

**[07:30-10:00] - Custom Challenges & Summary**
```
"Creating a custom challenge:

1. Define the challenge struct:
```go
type MyCustomChallenge struct {
    challenge.BaseChallenge
}

func (c *MyCustomChallenge) Execute(
    ctx context.Context) *challenge.Result {
    
    result := challenge.NewResult(c.ID())
    
    // Your verification logic here
    resp, err := http.Get('http://localhost:8080/api/v1/health')
    result.Assert('health returns 200', resp.StatusCode == 200)
    
    return result
}
```

2. Register it in register.go:
```go
registry.Register(&MyCustomChallenge{
    BaseChallenge: challenge.NewBaseChallenge(
        'CUSTOM-001', 'My Custom Check',
    ),
})
```

3. Run it:
```bash
curl -X POST http://localhost:8080/api/v1/challenges/CUSTOM-001/run
```

All challenge operations must go through the running
catalog-api service. Never use custom scripts or curl
directly against the database - always use the API
as an end user would.

Best Practices:
- Run challenges after every deployment
- Use individual challenges for quick verification
- Reserve RunAll for comprehensive regression checks
- Monitor challenge trends over time
- Add custom challenges for your specific requirements

That's the Challenge System! In Module 14, we'll
set up a development environment from scratch."
```

---

## Module 14: Development Setup (15 minutes)

### Learning Objectives
- Clone the repository and initialize all submodules
- Set up the Go backend development environment
- Set up the React frontend development environment
- Run the full test suite across all components
- Understand code style conventions and contribution guidelines

### Prerequisites
- Go 1.25 or later installed
- Node.js 18+ and npm
- Git with submodule support
- Code editor (VS Code recommended)
- Podman for container operations

### Script

**[00:00-03:00] - Cloning & Submodule Initialization**
```
"Welcome to Module 14! Let's set up a complete
development environment for Catalogizer.

Clone the repository:
```bash
git clone https://github.com/vasic-digital/Catalogizer.git
cd Catalogizer
```

Initialize all 41 submodules:
```bash
scripts/safe_submodule_init.sh
```

(Never run `git submodule update --recursive` through `constitution/`; the script wraps the safe sequence from docs/GOVERNANCE.md.)

This downloads all Go modules, TypeScript libraries,
and supporting tools. It may take a few minutes on
the first run.

Verify submodules:
```bash
git submodule status
```

You should see 41 entries, each with a commit hash
and path. No entries should have a '-' prefix
(which indicates uninitialized submodules).

Project structure overview:
- catalog-api/     : Go backend (Gin framework)
- catalog-web/     : React frontend (Vite + TypeScript)
- catalogizer-desktop/ : Tauri desktop app
- catalogizer-android/ : Android mobile app
- catalogizer-androidtv/ : Android TV app
- installer-wizard/ : Tauri setup wizard
- 22 Go submodules (Auth/, Cache/, Database/, etc.)
- 9 TypeScript submodules (UI-Components-React/, etc.)
- HelixQA and supporting AI submodules

[Show the directory listing with submodule folders]"
```

**[03:00-06:00] - Backend Setup (Go)**
```
"Let's set up the Go backend.

Prerequisites check:
```bash
go version    # Should be 1.25+
```

The Go modules use 'replace' directives in
catalog-api/go.mod to point to local submodule paths.
This means you don't need to publish modules to
work on them locally.

Build the backend:
```bash
cd catalog-api
go build -o catalog-api main.go
```

Create your .env file:
```bash
cp .env.example .env
```

Edit .env with your settings:
```
PORT=8080
GIN_MODE=debug
DB_TYPE=sqlite
JWT_SECRET=your-dev-secret-key-at-least-32-chars
ADMIN_PASSWORD=admin123
```

Run the development server:
```bash
go run main.go
```

The server starts and writes its port to .service-port.
This file is read by the frontend for API proxying.

Verify it's running:
```bash
curl http://localhost:8080/api/v1/health
```

For database, SQLite is used by default in development.
No setup needed - the database file is created
automatically. For PostgreSQL (production), set
DB_TYPE=postgres and provide connection details."
```

**[06:00-09:00] - Frontend Setup (React/TypeScript)**
```
"Now let's set up the React frontend.

Prerequisites:
```bash
node --version  # Should be 18+
npm --version
```

Install dependencies:
```bash
cd catalog-web
npm install
```

This also installs linked submodule packages via
file:../ references in package.json.

Start the development server:
```bash
npm run dev
```

The frontend starts on port 3000 and automatically
proxies /api requests to the catalog-api backend
by reading ../catalog-api/.service-port.

NOTE: Kill any process on port 3000 first:
```bash
ss -tlnp | grep :3000
```

Open http://localhost:3000 in your browser.

Key development tools:
- Hot Module Replacement (HMR) for instant updates
- Path aliases (@/components, @/hooks, @/lib, etc.)
- React Query DevTools for server state inspection
- TypeScript strict mode for type safety

Frontend structure:
- src/pages/      : Route-level components
- src/components/ : Reusable UI components
- src/hooks/      : Custom React hooks
- src/services/   : API service layer
- src/store/      : Zustand client state
- src/types/      : TypeScript type definitions

[Show the running frontend with hot reload]"
```

**[09:00-12:00] - Running Tests**
```
"Testing is critical. Let's run the full test suite.

Backend tests (with resource limits):
```bash
cd catalog-api
GOMAXPROCS=3 go test ./... -p 2 -parallel 2
```

Resource limits are mandatory - the host machine
runs other processes and we must stay under 30-40%
CPU/memory usage.

Run a single test:
```bash
go test -v -run TestMediaDetection ./internal/media/detector/
```

Frontend tests:
```bash
cd catalog-web
npm run test           # Single run (Vitest)
npm run test:watch     # Watch mode
npm run test:coverage  # With coverage report
```

End-to-end tests:
```bash
npm run test:e2e       # Playwright E2E
```

Lint and type checking:
```bash
npm run lint
npm run type-check
```

Full system test:
```bash
./scripts/run-all-tests.sh
```

Current test counts:
- Go: 44 packages, all passing
- Frontend: 130 test files, 2330+ tests
- Installer: 19 test files, 178 tests
- Security: govulncheck 0 vulnerabilities

[Show running go test and npm test with output]"
```

**[12:00-15:00] - Code Style & Contributing**
```
"Catalogizer follows strict code conventions.

Go conventions:
- Constructor injection via NewService() functions
- Error wrapping with context
- Table-driven tests in *_test.go files beside source
- Package names: lowercase, single word
- Test helpers in internal/tests/test_helper.go

TypeScript conventions:
- PascalCase for components
- camelCase for functions and variables
- Zod for runtime validation
- React Hook Form for forms
- Strict TypeScript (no 'any')

Kotlin conventions:
- MVVM architecture
- Result sealed classes for error handling
- Room for offline data
- Coroutines for async operations

Configuration precedence:
env vars > .env file > config.json > defaults

PostCSS note:
postcss.config.js MUST use module.exports (CommonJS)
for Node 18 compatibility.

Contributing workflow:
1. Create a feature branch
2. Write tests first (TDD encouraged)
3. Implement the feature
4. Run the full test suite
5. Ensure zero warnings, zero errors
6. Submit a pull request

Container builds are mandatory for releases:
```bash
./scripts/release-build.sh --container --force --skip-tests
```

Never build directly on bare metal for production.

That's development setup! In Module 15, we'll
explore API integration with the TypeScript client."
```

---

## Module 15: API Integration (12 minutes)

### Learning Objectives
- Install and configure the Catalogizer API client library
- Authenticate and manage JWT tokens
- Perform CRUD operations on media entities
- Subscribe to real-time WebSocket events
- Handle errors, rate limits, and pagination

### Prerequisites
- Node.js 18+ or TypeScript project
- Running catalog-api server
- Valid user credentials

### Script

**[00:00-02:30] - API Client Setup**
```
"Welcome to Module 15! The Catalogizer API client
makes integration straightforward.

Install the TypeScript client:
```bash
npm install @vasic-digital/catalogizer-api-client
```

Or if working within the monorepo, it's linked via
file:../ in catalog-web/package.json.

Basic setup:
```typescript
import { CatalogizerClient } from '@vasic-digital/catalogizer-api-client';

const client = new CatalogizerClient({
  baseUrl: 'http://localhost:8080',
  // Optional: provide token if already authenticated
  token: 'your-jwt-token',
});
```

The client handles:
- Automatic token refresh on 401 responses
- Brotli/gzip content negotiation
- HTTP/3 when available
- Request retry with exponential backoff
- Type-safe responses with TypeScript generics

All API routes are under /api/v1/ prefix."
```

**[02:30-05:00] - Authentication & CRUD Operations**
```
"Authenticate to get a JWT token:

```typescript
// Login
const auth = await client.auth.login({
  username: 'admin',
  password: 'your-password',
});
// auth.token and auth.refreshToken are stored internally

// Check current user
const me = await client.auth.me();
console.log(me.username, me.role);
```

CRUD Operations on Media Items:

```typescript
// List all movies
const movies = await client.media.list({
  type: 'movie',
  page: 1,
  pageSize: 20,
  sort: 'title',
});

// Get a specific item
const movie = await client.media.get(42);

// Search across all media types
const results = await client.media.search({
  query: 'Matrix',
  types: ['movie', 'tv_show'],
  minRating: 7.0,
});

// Collections
const collections = await client.collections.list();
const myList = await client.collections.create({
  name: 'Watch Later',
  description: 'Movies to watch this weekend',
});
await client.collections.addItem(myList.id, movie.id);

// Storage roots
const roots = await client.storage.listRoots();

// Trigger a scan
const scan = await client.storage.scan(roots[0].id);
```

All responses are fully typed. TypeScript catches
errors at compile time if you use wrong field names
or types.

[Show IDE autocompletion with typed responses]"
```

**[05:00-07:30] - WebSocket Events**
```
"Real-time events let your application react to
changes instantly.

```typescript
import { CatalogizerWebSocket } from '@vasic-digital/websocket-client';

const ws = new CatalogizerWebSocket({
  url: 'ws://localhost:8080/ws',
  token: auth.token,
  reconnect: true,
  reconnectInterval: 3000,
});

// Subscribe to events
ws.on('scan:progress', (data) => {
  console.log('Scan progress:', data.percentage);
});

ws.on('scan:complete', (data) => {
  console.log('Scan found', data.filesDetected, 'files');
});

ws.on('media:added', (data) => {
  console.log('New media:', data.title);
  // Update your UI
});

ws.on('media:updated', (data) => {
  console.log('Updated:', data.title);
});

// React hook (for React applications)
import { useWebSocket } from '@vasic-digital/websocket-client/react';

function ScanProgress() {
  const { lastMessage } = useWebSocket('scan:progress');
  return <ProgressBar value={lastMessage?.percentage} />;
}
```

The WebSocket client handles:
- Automatic reconnection with exponential backoff
- Token refresh on connection
- Message queuing during disconnection
- Heartbeat/ping-pong for connection health

Available event channels:
- scan:progress, scan:complete, scan:error
- media:added, media:updated, media:deleted
- collection:updated
- system:health, system:alert"
```

**[07:30-10:00] - Error Handling & Rate Limits**
```
"Robust error handling is essential for integration.

```typescript
try {
  const movie = await client.media.get(999);
} catch (error) {
  if (error.status === 404) {
    console.log('Movie not found');
  } else if (error.status === 401) {
    console.log('Token expired, refreshing...');
    // Client handles this automatically
  } else if (error.status === 429) {
    console.log('Rate limited, retry after:', 
      error.retryAfter, 'seconds');
  } else {
    console.error('Unexpected error:', error.message);
  }
}
```

Rate limit headers in every response:
- X-RateLimit-Limit: Maximum requests per window
- X-RateLimit-Remaining: Requests left
- X-RateLimit-Reset: Window reset timestamp

The client respects these automatically and backs
off when approaching limits.

Pagination:

```typescript
// First page
const page1 = await client.media.list({
  page: 1,
  pageSize: 50,
});
// page1.total, page1.page, page1.pageSize, page1.items

// Iterate all pages
async function* allMedia() {
  let page = 1;
  let hasMore = true;
  while (hasMore) {
    const result = await client.media.list({
      page, pageSize: 100,
    });
    yield* result.items;
    hasMore = page * 100 < result.total;
    page++;
  }
}

for await (const item of allMedia()) {
  console.log(item.title);
}
```"
```

**[10:00-12:00] - Integration Examples & Summary**
```
"Common integration patterns:

1. Dashboard Widget:
```typescript
const stats = await client.media.stats();
// { totalMovies: 450, totalShows: 89, totalMusic: 2300 }
```

2. Webhook Integration:
```typescript
// Register webhook for new media events
await client.webhooks.register({
  url: 'https://yourapp.com/catalogizer-hook',
  events: ['media:added', 'scan:complete'],
});
```

3. Batch Operations:
```typescript
// Add multiple items to collection
await client.collections.addItems(
  collectionId,
  [itemId1, itemId2, itemId3]
);
```

4. Export Data:
```typescript
const allMedia = await client.media.list({
  pageSize: 1000, sort: 'title',
});
// Transform and export to your format
```

API Documentation:
- Full OpenAPI/Swagger spec at /api/v1/docs
- TypeScript types exported from the client package
- Examples in catalogizer-api-client/examples/

Summary:
- Type-safe client library with auto-completion
- JWT auth with automatic token refresh
- Real-time updates via WebSocket
- Graceful error handling and rate limit respect
- Pagination support for large collections

In Module 16, we'll cover deploying Catalogizer
to production with Docker Compose."
```

---

## Module 16: Deployment and Operations (15 minutes)

### Learning Objectives
- Deploy Catalogizer using production Docker Compose
- Configure PostgreSQL for production use
- Set up Nginx as a reverse proxy with SSL
- Implement backup and restore procedures
- Scale the deployment for larger collections

### Prerequisites
- Linux server with Podman installed
- Domain name with DNS configured (for SSL)
- Basic understanding of container orchestration
- SSH access to the target server

### Script

**[00:00-03:00] - Production Docker Compose**
```
"Welcome to Module 16! Let's deploy Catalogizer
to production.

Catalogizer uses Podman exclusively - no Docker.
The production stack is defined in docker-compose.yml.

Review the production compose file:
```bash
podman-compose -f docker-compose.yml config --quiet
```

The stack includes:
- catalog-api: Go backend with HTTP/3
- catalog-web: React frontend (served by Nginx)
- PostgreSQL: Production database
- Redis: Caching layer
- Nginx: Reverse proxy with SSL

Start with environment configuration:
```bash
cp .env.example .env
nano .env
```

Critical production variables:
```
# Database
DB_TYPE=postgres
DB_HOST=localhost
DB_PORT=5433
DB_NAME=catalogizer
DB_USER=catalogizer
DB_PASSWORD=<strong-random-password>

# Authentication
JWT_SECRET=<64-char-random-string>
ADMIN_PASSWORD=<strong-admin-password>

# Server
GIN_MODE=release
PORT=8080

# Optional metadata providers
TMDB_API_KEY=your_key
OMDB_API_KEY=your_key
```

Generate strong secrets:
```bash
openssl rand -base64 64  # JWT_SECRET
openssl rand -base64 32  # DB_PASSWORD
```

[Show the .env file being configured]"
```

**[03:00-06:00] - PostgreSQL Setup**
```
"PostgreSQL is the production database.

Start the database container:
```bash
podman run -d --name catalogizer-postgres \
  --cpus=1 --memory=2g \
  -e POSTGRES_DB=catalogizer \
  -e POSTGRES_USER=catalogizer \
  -e POSTGRES_PASSWORD=<your-db-password> \
  -p 5433:5432 \
  -v catalogizer-pgdata:/var/lib/postgresql/data \
  docker.io/library/postgres:16-alpine
```

Note the resource limits (--cpus=1 --memory=2g).
This is mandatory - total container budget is
4 CPUs and 8 GB RAM across all containers.

Verify the database:
```bash
podman exec catalogizer-postgres \
  psql -U catalogizer -d catalogizer -c 'SELECT 1;'
```

The catalog-api automatically runs migrations on
startup. It supports 9 migration versions covering:
- Core tables (users, settings, storage roots)
- File tracking and scan history
- Media entity tables (media_items, media_files)
- Performance indexes
- Deduplication of media_files

Dialect abstraction handles the differences:
- SQLite: INSERT OR IGNORE, ?, boolean 0/1
- PostgreSQL: ON CONFLICT DO NOTHING, $1/$2, TRUE/FALSE

The database.DB wrapper auto-rewrites queries,
so all code works with both dialects transparently."
```

**[06:00-09:00] - Nginx Reverse Proxy & SSL**
```
"Nginx serves the frontend and proxies API requests.

The Nginx configuration is in config/nginx.conf.
Do NOT move this file - Docker Compose volume
mounts reference this path.

SSL with Let's Encrypt:
```bash
# Install certbot
sudo apt install certbot

# Obtain certificate
sudo certbot certonly --standalone \
  -d catalogizer.yourdomain.com
```

Update config/nginx.conf:
```nginx
server {
    listen 80;
    server_name catalogizer.yourdomain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    listen 443 quic reuseport;
    server_name catalogizer.yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/catalogizer.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/catalogizer.yourdomain.com/privkey.pem;

    # HTTP/3 advertisement
    add_header Alt-Svc 'h3=\":443\"; ma=86400';

    # Brotli compression
    brotli on;
    brotli_types text/html application/json application/javascript;

    # Frontend
    location / {
        root /usr/share/nginx/html;
        try_files $uri $uri/ /index.html;
    }

    # API proxy
    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # WebSocket proxy
    location /ws {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
    }
}
```

Start the full stack:
```bash
podman-compose -f docker-compose.yml up -d
```

Verify everything is running:
```bash
podman-compose -f docker-compose.yml ps
```

[Show the running containers and health checks]"
```

**[09:00-12:00] - Backup & Restore**
```
"Regular backups are essential. Here's how to
protect your data.

PostgreSQL Backup:
```bash
# Full database dump
podman exec catalogizer-postgres \
  pg_dump -U catalogizer catalogizer | \
  gzip > backup-$(date +%Y%m%d).sql.gz

# Schedule daily backups via cron
0 3 * * * /path/to/backup-script.sh
```

Restore from backup:
```bash
# Stop the API first
podman stop catalog-api

# Restore
gunzip -c backup-20260403.sql.gz | \
  podman exec -i catalogizer-postgres \
  psql -U catalogizer catalogizer

# Restart the API
podman start catalog-api
```

Media Files:
Media files live on your storage roots (NAS, local
disk, etc.). These are not in the database - only
metadata and file paths are stored.

Backup strategy:
1. Database: Daily pg_dump (small, fast)
2. Configuration: Version control .env.example,
   config files, compose files
3. Media: Use your NAS RAID or backup solution
4. Certificates: Copy /etc/letsencrypt/

Data volume protection:
The PostgreSQL data lives in a named volume:
```bash
podman volume inspect catalogizer-pgdata
```

Never delete this volume without backing up first.

[Show running a backup and verifying the dump file]"
```

**[12:00-15:00] - Scaling & Operations Summary**
```
"Scaling for larger collections:

Database Tuning:
```
# PostgreSQL performance settings
shared_buffers = 512MB
effective_cache_size = 1.5GB
work_mem = 16MB
maintenance_work_mem = 128MB
```

Connection Pool Tuning:
catalog-api defaults: MaxOpen=25, MaxIdle=10,
MaxLifetime=5m, MaxIdleTime=3m.
Adjust via config.json for higher concurrency.

Caching with Redis:
Redis dramatically reduces database load:
```bash
podman run -d --name catalogizer-redis \
  --cpus=0.5 --memory=512m \
  -v $(pwd)/config/redis.conf:/usr/local/etc/redis/redis.conf \
  docker.io/library/redis:7-alpine
```

Monitoring (from Module 11):
Always run Prometheus + Grafana alongside production.

Operational Checklist:
[ ] Daily database backups verified
[ ] SSL certificate auto-renewal configured
[ ] Monitoring dashboards reviewed weekly
[ ] Security scans run before releases
[ ] Container images updated monthly
[ ] Log rotation configured
[ ] Resource limits enforced on all containers

Container resource budget (mandatory):
- PostgreSQL: --cpus=1 --memory=2g
- catalog-api: --cpus=2 --memory=4g
- catalog-web (Nginx): --cpus=1 --memory=2g
- Redis: --cpus=0.5 --memory=512m
- Total: max 4 CPUs, 8 GB RAM

That's production deployment! In Module 17, we'll
cover troubleshooting common issues."
```

---

## Module 17: Troubleshooting (10 minutes)

### Learning Objectives
- Diagnose the most common Catalogizer issues
- Use diagnostic commands to identify root causes
- Resolve database, network, and performance problems
- Analyze logs effectively
- Know where to get help when stuck

### Prerequisites
- Access to the running Catalogizer deployment
- Terminal access to the host machine
- Basic familiarity with Podman and system tools

### Script

**[00:00-02:30] - Common Issues & Quick Fixes**
```
"Welcome to Module 17! Let's tackle the most
frequent issues you'll encounter.

Issue 1: Frontend can't reach the API
Symptoms: Network errors in browser console,
'Connection refused' on /api calls.

Diagnosis:
```bash
# Check if API is running
curl http://localhost:8080/api/v1/health

# Check .service-port file
cat catalog-api/.service-port

# Check if port is in use by another process
ss -tlnp | grep :8080
ss -tlnp | grep :3000
```

Fix: Ensure catalog-api is running and .service-port
matches the actual port. Kill any process on port 3000
(Bear Messenger commonly occupies this port).

Issue 2: 'Database is locked' (SQLite)
Symptoms: 500 errors during concurrent writes.

Fix: SQLite WAL mode should be enabled automatically.
Verify:
```bash
sqlite3 catalogizer.db 'PRAGMA journal_mode;'
# Should return: wal
```

If not 'wal', the PRAGMA is not executing. Check
database/connection.go for the explicit WAL pragma.

Issue 3: Scan finds no files
Symptoms: Scan completes but 0 files detected.

Diagnosis:
```bash
# Verify storage root path exists and is accessible
ls -la /path/to/your/media

# For SMB/NFS mounts, check if mounted
mount | grep /path/to/media
```

Fix: Ensure the storage root path is correct and
the catalog-api process has read permissions."
```

**[02:30-05:00] - Diagnostic Commands**
```
"These commands help pinpoint problems quickly.

System Health:
```bash
# Overall system load (stay under 30-40%)
cat /proc/loadavg

# Container resource usage
podman stats --no-stream

# Disk space
df -h /path/to/media
df -h /path/to/database
```

API Diagnostics:
```bash
# Health check with timing
curl -w 'Total: %{time_total}s\n' \
  http://localhost:8080/api/v1/health

# Check all API endpoints respond
curl -s http://localhost:8080/api/v1/health/detailed | jq .

# Count active WebSocket connections
curl -s http://localhost:8080/metrics | \
  grep websocket_connections
```

Database Diagnostics:
```bash
# PostgreSQL: Check active connections
podman exec catalogizer-postgres \
  psql -U catalogizer -d catalogizer \
  -c 'SELECT count(*) FROM pg_stat_activity;'

# PostgreSQL: Check table sizes
podman exec catalogizer-postgres \
  psql -U catalogizer -d catalogizer \
  -c 'SELECT relname, pg_size_pretty(pg_total_relation_size(relid))
      FROM pg_catalog.pg_statio_user_tables
      ORDER BY pg_total_relation_size(relid) DESC;'

# SQLite: Check integrity
sqlite3 catalogizer.db 'PRAGMA integrity_check;'
```

Container Diagnostics:
```bash
# View container logs
podman logs --tail 100 catalog-api
podman logs --tail 100 catalogizer-postgres

# Check container health
podman inspect --format='{{.State.Health.Status}}' catalog-api

# Restart a misbehaving container
podman restart catalog-api
```"
```

**[05:00-07:30] - Database & Network Issues**
```
"Database Issues:

Problem: Migration fails on startup
```bash
# Check migration status in logs
podman logs catalog-api 2>&1 | grep -i migration
```
Migrations run automatically. If one fails, check:
- PostgreSQL: Are all 9 migration versions applied?
- Column mismatches between SQLite and PostgreSQL
- The dialect abstraction rewrites queries automatically

Problem: Slow queries
```bash
# PostgreSQL: Enable slow query logging
# In postgresql.conf:
log_min_duration_statement = 1000  # log queries > 1s

# Check for missing indexes
podman exec catalogizer-postgres \
  psql -U catalogizer -d catalogizer \
  -c 'SELECT * FROM pg_stat_user_indexes
      WHERE idx_scan = 0;'
```

Network Issues:

Problem: NAS (SMB/NFS) connection drops
```bash
# Test NAS connectivity
ping synology.local
smbclient //synology.local/media -U user -c 'ls'
```
The SMB client has built-in circuit breaker with
exponential backoff retry. Check logs for retry
patterns.

Problem: WebSocket disconnections
```bash
# Monitor WebSocket connections
podman logs catalog-api 2>&1 | grep -i websocket
```
The WebSocket handler uses sync.Once for safe
shutdown. Clients should auto-reconnect."
```

**[07:30-09:00] - Performance Debugging**
```
"When things are slow, here's how to investigate.

High CPU:
```bash
# Find what's consuming CPU
podman stats --no-stream
cat /proc/loadavg

# If Go CPU is high, check goroutine count
curl -s http://localhost:8080/metrics | grep goroutines
```
A goroutine count above 1000 suggests a leak.
The Memory module provides leak detection.

High Memory:
```bash
# Check memory usage
podman stats --no-stream --format \
  'table {{.Name}}\t{{.MemUsage}}'

# Go memory details
curl -s http://localhost:8080/metrics | grep memstats
```

Slow Scans:
- Large NAS: 85K files takes ~25 minutes
- Scan progress is reported every 5 seconds
- Use incremental scans for daily updates
- Schedule full scans during off-peak hours

Slow API Responses:
```bash
# Test endpoint latency
curl -w '\nTime: %{time_total}s\n' \
  http://localhost:8080/api/v1/media?page=1&pageSize=20

# Check if Redis caching is working
curl -s http://localhost:8080/metrics | grep cache_hit
```

If responses are slow without caching enabled,
consider enabling Redis (Module 16)."
```

**[09:00-10:00] - Getting Help & Summary**
```
"When you're stuck, here's where to go:

Documentation:
- CLAUDE.md: Comprehensive project reference
- docs/ directory: Architecture, plans, guides
- ARCHITECTURE.md in each submodule
- API docs at /api/v1/docs (when running)

Logs to collect for bug reports:
1. catalog-api logs: podman logs catalog-api
2. Database logs: podman logs catalogizer-postgres
3. Browser console: F12 > Console tab
4. System info: uname -a, go version, node --version
5. Configuration: .env (redact secrets!)

Diagnostic one-liner:
```bash
echo '=== Health ===' && \
curl -s http://localhost:8080/api/v1/health | jq . && \
echo '=== Containers ===' && \
podman ps --format 'table {{.Names}}\t{{.Status}}' && \
echo '=== Load ===' && \
cat /proc/loadavg && \
echo '=== Disk ===' && \
df -h / | tail -1
```

Run the challenge system to verify everything:
```bash
curl -X POST http://localhost:8080/api/v1/challenges/CH-001/run \
  -H 'Authorization: Bearer <token>'
```

Quick Troubleshooting Checklist:
[ ] Is catalog-api running? (check health endpoint)
[ ] Is the database accessible? (check connection)
[ ] Are ports free? (check ss -tlnp)
[ ] Are containers healthy? (podman ps)
[ ] Are resource limits respected? (podman stats)
[ ] Are logs showing errors? (podman logs)

That concludes our troubleshooting module and the
entire extended course series! You now have the
knowledge to install, configure, operate, develop
for, and troubleshoot Catalogizer across all
platforms. Happy cataloging!"
```

---

## Production Notes

### Recording Setup
- **Resolution:** 1920x1080 minimum
- **Frame Rate:** 30fps
- **Audio:** Clear narration, minimal background noise
- **Terminal:** Large font (16pt+), high contrast theme

### Visual Aids
- Use callouts for important commands
- Highlight configuration files
- Show split-screen for code + terminal
- Include zoom-in on critical details

### Interactive Elements
- Pause points for viewer practice
- Quiz questions at module ends
- Downloadable configuration templates
- Cheat sheet PDF

### Accessibility
- Closed captions for all videos
- Transcript available
- Keyboard shortcuts documented
- Screen reader compatible code
