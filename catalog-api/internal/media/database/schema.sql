-- Media Detection and Metadata Database Schema
-- Using SQLite with SQLCipher encryption

-- Media Types enumeration table
CREATE TABLE IF NOT EXISTS media_types (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE, -- movie, tv_show, music, game, software, training, concert, documentary, etc.
    description TEXT,
    detection_patterns TEXT, -- JSON array of file patterns and directory structures
    metadata_providers TEXT, -- JSON array of supported providers
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Detected media items (aggregated view of content)
CREATE TABLE IF NOT EXISTS media_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    original_title TEXT,
    year INTEGER,
    description TEXT,
    genre TEXT, -- JSON array
    director TEXT,
    cast_crew TEXT, -- JSON object
    rating REAL,
    runtime INTEGER, -- in minutes
    language TEXT,
    country TEXT,
    status TEXT DEFAULT 'active', -- active, archived, missing
    first_detected DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_type_id) REFERENCES media_types(id)
);

-- External metadata from various providers
CREATE TABLE IF NOT EXISTS external_metadata (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL,
    provider TEXT NOT NULL, -- imdb, tmdb, tvdb, musicbrainz, igdb, etc.
    external_id TEXT NOT NULL,
    data TEXT NOT NULL, -- JSON blob of all metadata from provider
    rating REAL,
    review_url TEXT,
    cover_url TEXT,
    trailer_url TEXT,
    last_fetched DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_item_id) REFERENCES media_items(id),
    UNIQUE(media_item_id, provider)
);

-- Directory analysis and detection
CREATE TABLE IF NOT EXISTS directory_analysis (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    directory_path TEXT NOT NULL UNIQUE,
    smb_root TEXT NOT NULL,
    media_item_id INTEGER,
    confidence_score REAL NOT NULL, -- 0.0 to 1.0
    detection_method TEXT NOT NULL, -- filename, structure, metadata, hybrid
    analysis_data TEXT, -- JSON with detection details
    last_analyzed DATETIME DEFAULT CURRENT_TIMESTAMP,
    files_count INTEGER DEFAULT 0,
    total_size INTEGER DEFAULT 0,
    FOREIGN KEY (media_item_id) REFERENCES media_items(id),
    INDEX idx_directory_path (directory_path),
    INDEX idx_smb_root (smb_root),
    INDEX idx_media_item (media_item_id)
);

-- Individual file versions and qualities
CREATE TABLE IF NOT EXISTS media_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    smb_root TEXT NOT NULL,
    filename TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    file_extension TEXT,
    quality_info TEXT, -- JSON: resolution, bitrate, codec, etc.
    language TEXT,
    subtitle_tracks TEXT, -- JSON array
    audio_tracks TEXT, -- JSON array
    duration INTEGER, -- in seconds
    checksum TEXT,
    virtual_smb_link TEXT, -- generated virtual SMB link
    direct_smb_link TEXT, -- direct SMB path
    last_verified DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_item_id) REFERENCES media_items(id),
    INDEX idx_media_item_files (media_item_id),
    INDEX idx_file_path (file_path),
    INDEX idx_quality (quality_info)
);

-- Quality comparison and ranking
CREATE TABLE IF NOT EXISTS quality_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE, -- 4K, 1080p, 720p, DVD, etc.
    resolution_width INTEGER,
    resolution_height INTEGER,
    min_bitrate INTEGER,
    max_bitrate INTEGER,
    preferred_codecs TEXT, -- JSON array
    quality_score INTEGER NOT NULL, -- higher = better quality
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Real-time monitoring and change tracking
CREATE TABLE IF NOT EXISTS change_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type TEXT NOT NULL, -- directory, file, metadata
    entity_id TEXT NOT NULL,
    change_type TEXT NOT NULL, -- created, updated, deleted, moved
    old_data TEXT, -- JSON
    new_data TEXT, -- JSON
    detected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    processed BOOLEAN DEFAULT FALSE,
    INDEX idx_change_type (change_type),
    INDEX idx_detected_at (detected_at),
    INDEX idx_processed (processed)
);

-- Media collection grouping (for series, albums, game series, etc.)
CREATE TABLE IF NOT EXISTS media_collections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    collection_type TEXT NOT NULL, -- tv_series, movie_franchise, album_discography, game_series
    description TEXT,
    total_items INTEGER DEFAULT 0,
    external_ids TEXT, -- JSON object with provider IDs
    cover_url TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Relationship between media items and collections
CREATE TABLE IF NOT EXISTS media_collection_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    collection_id INTEGER NOT NULL,
    media_item_id INTEGER NOT NULL,
    sequence_number INTEGER, -- episode number, album track, etc.
    season_number INTEGER, -- for TV shows
    release_order INTEGER,
    FOREIGN KEY (collection_id) REFERENCES media_collections(id),
    FOREIGN KEY (media_item_id) REFERENCES media_items(id),
    UNIQUE(collection_id, media_item_id)
);

-- User preferences and custom metadata
CREATE TABLE IF NOT EXISTS user_metadata (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL DEFAULT 0,
    user_rating REAL,
    watched_status TEXT, -- unwatched, watching, completed, dropped
    watched_date DATETIME,
    personal_notes TEXT,
    tags TEXT, -- JSON array
    favorite BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_item_id) REFERENCES media_items(id)
);

-- Detection rules and patterns
CREATE TABLE IF NOT EXISTS detection_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type_id INTEGER NOT NULL,
    rule_name TEXT NOT NULL,
    rule_type TEXT NOT NULL, -- filename_pattern, directory_structure, file_analysis
    pattern TEXT NOT NULL, -- regex or JSON structure
    confidence_weight REAL DEFAULT 1.0,
    enabled BOOLEAN DEFAULT TRUE,
    priority INTEGER DEFAULT 0, -- higher priority rules run first
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_type_id) REFERENCES media_types(id)
);

-- Insert comprehensive media types covering all possible content
INSERT OR IGNORE INTO media_types (name, description, detection_patterns, metadata_providers) VALUES
-- Video Content
('movie', 'Feature films and movies', '["*.mp4", "*.mkv", "*.avi", "*.mov", "*movie*", "*film*", "*bluray*", "*brrip*", "*dvdrip*"]', '["imdb", "tmdb", "rotten_tomatoes"]'),
('tv_show', 'Television series and episodes', '["*S[0-9][0-9]E[0-9][0-9]*", "*season*", "*episode*", "*series*", "*complete*"]', '["imdb", "tmdb", "tvdb", "trakt"]'),
('anime', 'Animated series and movies from Japan', '["*anime*", "*[0-9][0-9][0-9][0-9]p*", "*subbed*", "*dubbed*", "*BD*"]', '["anidb", "myanimelist", "kitsu"]'),
('documentary', 'Documentary films and series', '["*documentary*", "*docu*", "*nature*", "*history*", "*science*"]', '["imdb", "tmdb", "bbc"]'),
('tv_movie', 'Made-for-television movies', '["*tv.movie*", "*television*", "*made.for.tv*"]', '["imdb", "tmdb"]'),
('short_film', 'Short films and clips', '["*short*", "*clip*", "*trailer*", "*teaser*"]', '["imdb", "tmdb", "youtube"]'),
('concert', 'Live music performances and concerts', '["*concert*", "*live*", "*performance*", "*tour*", "*festival*"]', '["musicbrainz", "setlistfm", "last.fm"]'),
('comedy_special', 'Stand-up comedy specials', '["*comedy*", "*standup*", "*stand.up*", "*special*"]', '["imdb", "tmdb", "netflix"]'),
('sports', 'Sports events and matches', '["*sports*", "*match*", "*game*", "*championship*", "*tournament*"]', '["espn", "sports_db"]'),

-- Audio Content
('music', 'Music albums and tracks', '["*.mp3", "*.flac", "*.wav", "*.m4a", "*album*", "*discography*", "*lossless*"]', '["musicbrainz", "spotify", "last.fm", "discogs"]'),
('podcast', 'Podcast episodes and series', '["*podcast*", "*episode*", "*.mp3", "*talk*", "*interview*"]', '["spotify", "apple_podcasts", "google_podcasts"]'),
('audiobook', 'Audio books and narrations', '["*audiobook*", "*narrated*", "*audio.book*", "*.m4b", "*unabridged*"]', '["audible", "goodreads", "librivox"]'),
('radio_show', 'Radio programs and shows', '["*radio*", "*show*", "*broadcast*", "*fm*", "*am*"]', '["radio_db", "tunein"]'),
('sound_effect', 'Sound effects and audio samples', '["*sfx*", "*sound*", "*effect*", "*sample*", "*loop*"]', '["freesound", "zapsplat"]'),

-- Gaming Content
('pc_game', 'PC video games', '["*.exe", "*.msi", "*setup*", "*install*", "*game*", "*pc*", "*steam*"]', '["igdb", "steam", "gog", "metacritic"]'),
('console_game', 'Console video games', '["*.iso", "*.rom", "*.bin", "*xbox*", "*ps4*", "*ps5*", "*nintendo*"]', '["igdb", "gamespot", "ign"]'),
('mobile_game', 'Mobile games and apps', '["*.apk", "*.ipa", "*mobile*", "*android*", "*ios*"]', '["google_play", "app_store", "igdb"]'),
('game_mod', 'Game modifications and mods', '["*mod*", "*patch*", "*addon*", "*expansion*", "*dlc*"]', '["nexusmods", "moddb"]'),
('emulator', 'Gaming emulators and ROMs', '["*emulator*", "*rom*", "*emu*", "*bios*"]', '["emulation_db"]'),

-- Software & Applications
('software', 'General applications and utilities', '["*.exe", "*.msi", "*.dmg", "*.deb", "*.rpm", "*setup*", "*install*"]', '["github", "sourceforge", "softpedia"]'),
('os', 'Operating systems and distributions', '["*.iso", "*windows*", "*linux*", "*macos*", "*ubuntu*", "*distro*"]', '["distrowatch", "microsoft", "apple"]'),
('driver', 'Hardware drivers and firmware', '["*driver*", "*firmware*", "*bios*", "*update*"]', '["manufacturer_sites"]'),
('plugin', 'Software plugins and extensions', '["*plugin*", "*extension*", "*addon*", "*.dll", "*.so"]', '["plugin_repositories"]'),
('portable_app', 'Portable applications', '["*portable*", "*no.install*", "*standalone*"]', '["portableapps"]'),

-- Educational & Training
('training', 'Professional training and courses', '["*training*", "*course*", "*tutorial*", "*lesson*", "*certification*"]', '["udemy", "coursera", "lynda"]'),
('language_learning', 'Language learning materials', '["*language*", "*learn*", "*speak*", "*rosetta*", "*babel*"]', '["duolingo", "babbel", "rosetta_stone"]'),
('academic', 'Academic lectures and materials', '["*lecture*", "*university*", "*college*", "*academic*", "*thesis*"]', '["coursera", "edx", "khan_academy"]'),
('tutorial', 'How-to guides and tutorials', '["*howto*", "*guide*", "*tutorial*", "*diy*", "*learn*"]', '["youtube", "instructables"]'),
('certification', 'Certification exam materials', '["*cert*", "*exam*", "*test*", "*practice*", "*prep*"]', '["certification_bodies"]'),

-- Books & Documents
('ebook', 'Electronic books', '["*.pdf", "*.epub", "*.mobi", "*.azw*", "*book*", "*novel*"]', '["goodreads", "amazon", "openlibrary"]'),
('comic', 'Comics and graphic novels', '["*.cbr", "*.cbz", "*.cb7", "*comic*", "*manga*", "*graphic*"]', '["comicvine", "marvel", "dc"]'),
('magazine', 'Digital magazines and periodicals', '["*magazine*", "*periodical*", "*issue*", "*monthly*", "*weekly*"]', '["magazine_db"]'),
('manual', 'User manuals and documentation', '["*manual*", "*guide*", "*documentation*", "*readme*", "*help*"]', '["manufacturer_sites"]'),
('research_paper', 'Academic papers and research', '["*paper*", "*research*", "*study*", "*journal*", "*article*"]', '["arxiv", "pubmed", "ieee"]'),

-- YouTube & Streaming Content
('youtube_video', 'YouTube videos and channels', '["*youtube*", "*yt*", "*youtuber*", "*channel*", "*vlog*"]', '["youtube_api", "social_blade"]'),
('twitch_stream', 'Twitch streams and clips', '["*twitch*", "*stream*", "*clip*", "*highlight*", "*vod*"]', '["twitch_api"]'),
('tiktok', 'TikTok videos and compilations', '["*tiktok*", "*tt*", "*compilation*", "*dance*", "*meme*"]', '["tiktok_api"]'),
('web_series', 'Web-based series and content', '["*web.series*", "*webseries*", "*online*", "*webcast*"]', '["imdb", "web_series_db"]'),

-- Adult Content (if applicable)
('adult', 'Adult entertainment content', '["*xxx*", "*adult*", "*18+*", "*nsfw*"]', '["adult_db"]'),

-- Archives & Collections
('archive', 'Archive collections and compilations', '["*archive*", "*collection*", "*compilation*", "*pack*", "*bundle*"]', '["archive_org"]'),
('backup', 'Backup files and system images', '["*backup*", "*image*", "*ghost*", "*clone*", "*snapshot*"]', '["backup_tools"]'),

-- Specialty Content
('karaoke', 'Karaoke tracks and videos', '["*karaoke*", "*cdg*", "*kar*", "*sing*"]', '["karafun", "karaoke_db"]'),
('ringtone', 'Ringtones and notification sounds', '["*ringtone*", "*ring*", "*tone*", "*notification*"]', '["zedge"]'),
('wallpaper', 'Desktop wallpapers and backgrounds', '["*wallpaper*", "*background*", "*desktop*", "*.jpg", "*.png"]', '["wallhaven", "unsplash"]'),
('font', 'Digital fonts and typography', '["*.ttf", "*.otf", "*font*", "*typeface*", "*typography*"]', '["google_fonts", "dafont"]'),
('3d_model', '3D models and assets', '["*.obj", "*.fbx", "*.blend", "*3d*", "*model*"]', '["sketchfab", "turbosquid"]'),
('template', 'Templates and design assets', '["*template*", "*.psd", "*.ai", "*design*", "*asset*"]', '["envato", "freepik"]'),

-- News & Media
('news', 'News broadcasts and reports', '["*news*", "*report*", "*broadcast*", "*bulletin*"]', '["news_api"]'),
('interview', 'Interviews and talks', '["*interview*", "*talk*", "*conversation*", "*q&a*"]', '["imdb", "youtube"]'),

-- Health & Fitness
('workout', 'Fitness and workout videos', '["*workout*", "*fitness*", "*exercise*", "*yoga*", "*pilates*"]', '["fitness_db"]'),
('meditation', 'Meditation and mindfulness content', '["*meditation*", "*mindfulness*", "*calm*", "*relax*"]', '["headspace", "calm"]'),

-- Technology & Programming
('code', 'Source code and programming projects', '["*.zip", "*.tar*", "*source*", "*code*", "*project*"]', '["github", "gitlab", "sourceforge"]'),
('presentation', 'Presentations and slides', '["*.ppt*", "*.key", "*presentation*", "*slides*", "*deck*"]', '["slideshare"]'),

-- Miscellaneous
('other', 'Unclassified or mixed content', '["*misc*", "*other*", "*mixed*", "*various*"]', '["custom"]'),
('sample', 'Sample or demo content', '["*sample*", "*demo*", "*preview*", "*trial*"]', '["custom"]');

-- Insert default quality profiles
INSERT OR IGNORE INTO quality_profiles (name, resolution_width, resolution_height, min_bitrate, quality_score) VALUES
('4K/UHD', 3840, 2160, 15000, 100),
('1080p', 1920, 1080, 5000, 80),
('720p', 1280, 720, 2500, 60),
('480p/DVD', 720, 480, 1000, 40),
('360p', 640, 360, 500, 20),
('Audio_Lossless', 0, 0, 1000, 90),
('Audio_320k', 0, 0, 320, 70),
('Audio_128k', 0, 0, 128, 30);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_media_items_type ON media_items(media_type_id);
CREATE INDEX IF NOT EXISTS idx_media_items_title ON media_items(title);
CREATE INDEX IF NOT EXISTS idx_media_items_year ON media_items(year);
CREATE INDEX IF NOT EXISTS idx_external_metadata_provider ON external_metadata(provider);
CREATE INDEX IF NOT EXISTS idx_media_files_size ON media_files(file_size);
CREATE INDEX IF NOT EXISTS idx_media_files_extension ON media_files(file_extension);

-- Views for common queries
CREATE VIEW IF NOT EXISTS media_overview AS
SELECT
    mi.id,
    mi.title,
    mi.year,
    mt.name as media_type,
    COUNT(mf.id) as file_count,
    SUM(mf.file_size) as total_size,
    MAX(mf.last_verified) as last_verified,
    GROUP_CONCAT(DISTINCT substr(mf.quality_info, 1, 20)) as available_qualities
FROM media_items mi
JOIN media_types mt ON mi.media_type_id = mt.id
LEFT JOIN media_files mf ON mi.id = mf.media_item_id
GROUP BY mi.id, mi.title, mi.year, mt.name;

CREATE VIEW IF NOT EXISTS duplicate_media AS
SELECT
    mi1.title,
    mi1.year,
    mt.name as media_type,
    COUNT(*) as duplicate_count,
    GROUP_CONCAT(mi1.id) as media_item_ids
FROM media_items mi1
JOIN media_types mt ON mi1.media_type_id = mt.id
WHERE EXISTS (
    SELECT 1 FROM media_items mi2
    WHERE mi2.title = mi1.title
    AND mi2.year = mi1.year
    AND mi2.media_type_id = mi1.media_type_id
    AND mi2.id != mi1.id
)
GROUP BY mi1.title, mi1.year, mi1.media_type_id
HAVING COUNT(*) > 1;