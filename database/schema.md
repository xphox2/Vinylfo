// This file contains database schema definitions and migration scripts for the Vinylfo project

// Database Schema for Vinylfo Application

// Album table:
// - id (primary key, auto-increment)
// - title (not null, unique)
// - artist (not null)
// - release_year
// - genre
// - cover_image_url
// - created_at
// - updated_at

// Track table:
// - id (primary key, auto-increment)
// - album_id (foreign key, not null, index)
// - title (not null)
// - duration (in seconds)
// - track_number
// - audio_file_url
// - created_at
// - updated_at

// PlaybackSession table:
// - id (primary key, auto-increment)
// - track_id (foreign key, not null, index)
// - start_time (when playback started)
// - end_time (when playback ended)
// - duration (in seconds)
// - progress (in seconds)
// - created_at
// - updated_at

// Relationships:
// - Album to Track: one-to-many (via album_id foreign key)
// - Track to PlaybackSession: one-to-many (via track_id foreign key)

// Constraints:
// - Indexes on frequently queried fields (album_id, track_id)
// - Proper foreign key constraints for referential integrity