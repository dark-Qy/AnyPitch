package media

import (
	"bytes"
	"database/sql"
	"errors"
	"image"
	"image/color"
	stdraw "image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	MaxUploadBytes = 6 << 20
	MaxPixels      = 25_000_000
	FullImageEdge  = 2048
	ThumbnailEdge  = 480
	MaxTeamBytes   = 5 << 30
)

var (
	ErrInvalidAlbum = errors.New("invalid album")
	ErrInvalidPhoto = errors.New("invalid photo")
	ErrForbidden    = errors.New("forbidden media operation")
	ErrStorageFull  = errors.New("team media storage is full")
)

type Album struct {
	ID                string `json:"id"`
	EventID           string `json:"event_id,omitempty"`
	SpecialActivityID string `json:"special_activity_id,omitempty"`
	SourceKind        string `json:"source_kind"`
	SourceTitle       string `json:"source_title"`
	PhotoCount        int    `json:"photo_count"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type Photo struct {
	ID               string `json:"id"`
	AlbumID          string `json:"album_id"`
	Caption          string `json:"caption"`
	UploaderPlayerID string `json:"uploader_player_id,omitempty"`
	UploaderName     string `json:"uploader_name"`
	UploaderRole     string `json:"uploader_role"`
	ContentType      string `json:"content_type"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	ByteSize         int64  `json:"byte_size"`
	CreatedAt        string `json:"created_at"`
}

type Actor struct {
	Role     string
	PlayerID string
}

type Service struct {
	db   *sql.DB
	root string
}

func NewService(db *sql.DB, root string) (*Service, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, ErrInvalidPhoto
	}
	for _, directory := range []string{filepath.Join(root, "photos"), filepath.Join(root, "thumbs")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, err
		}
	}
	return &Service{db: db, root: root}, nil
}

func (s *Service) ListAlbums(teamID string) ([]Album, error) {
	rows, err := s.db.Query(`SELECT a.id, COALESCE(a.event_id, ''), COALESCE(a.special_activity_id, ''),
		CASE WHEN a.event_id IS NOT NULL THEN 'event' ELSE 'special_activity' END,
		COALESCE(e.title, special.name), COUNT(p.id), a.created_at, a.updated_at
		FROM albums a
		LEFT JOIN events e ON e.id = a.event_id
		LEFT JOIN special_activities special ON special.id = a.special_activity_id
		LEFT JOIN photos p ON p.album_id = a.id AND p.deleted_at = ''
		WHERE a.team_id = ?
		GROUP BY a.id
		ORDER BY a.updated_at DESC, a.id`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	albums := []Album{}
	for rows.Next() {
		var album Album
		if err := rows.Scan(&album.ID, &album.EventID, &album.SpecialActivityID, &album.SourceKind, &album.SourceTitle, &album.PhotoCount, &album.CreatedAt, &album.UpdatedAt); err != nil {
			return nil, err
		}
		albums = append(albums, album)
	}
	return albums, rows.Err()
}

func (s *Service) EnsureAlbum(teamID, eventID, specialActivityID string) (Album, error) {
	if (eventID == "") == (specialActivityID == "") {
		return Album{}, ErrInvalidAlbum
	}
	if eventID != "" {
		var exists int
		err := s.db.QueryRow(`SELECT 1 FROM events WHERE id = ? AND team_id = ?`, eventID, teamID).Scan(&exists)
		if err != nil {
			return Album{}, ErrInvalidAlbum
		}
	} else {
		var exists int
		err := s.db.QueryRow(`SELECT 1 FROM special_activities WHERE id = ? AND team_id = ?`, specialActivityID, teamID).Scan(&exists)
		if err != nil {
			return Album{}, ErrInvalidAlbum
		}
	}
	var id string
	if eventID != "" {
		err := s.db.QueryRow(`SELECT id FROM albums WHERE team_id = ? AND event_id = ?`, teamID, eventID).Scan(&id)
		if err == nil {
			return s.album(teamID, id)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return Album{}, err
		}
	} else {
		err := s.db.QueryRow(`SELECT id FROM albums WHERE team_id = ? AND special_activity_id = ?`, teamID, specialActivityID).Scan(&id)
		if err == nil {
			return s.album(teamID, id)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return Album{}, err
		}
	}
	now := nowISO()
	id = uuid.NewString()
	_, err := s.db.Exec(`INSERT INTO albums (id, team_id, event_id, special_activity_id, created_at, updated_at) VALUES (?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?)`, id, teamID, eventID, specialActivityID, now, now)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "unique") {
		return Album{}, err
	}
	if err != nil {
		if eventID != "" {
			err = s.db.QueryRow(`SELECT id FROM albums WHERE team_id = ? AND event_id = ?`, teamID, eventID).Scan(&id)
		} else {
			err = s.db.QueryRow(`SELECT id FROM albums WHERE team_id = ? AND special_activity_id = ?`, teamID, specialActivityID).Scan(&id)
		}
		if err != nil {
			return Album{}, err
		}
	}
	return s.album(teamID, id)
}

func (s *Service) ListPhotos(teamID, albumID string) ([]Photo, error) {
	if _, err := s.album(teamID, albumID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT p.id, p.album_id, p.caption, COALESCE(p.uploader_player_id, ''), COALESCE(players.name, CASE p.uploader_role WHEN 'coach' THEN '教练' ELSE '队员' END), p.uploader_role, p.content_type, p.width, p.height, p.byte_size, p.created_at FROM photos p LEFT JOIN players ON players.id = p.uploader_player_id WHERE p.album_id = ? AND p.deleted_at = '' ORDER BY p.created_at DESC, p.id`, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	photos := []Photo{}
	for rows.Next() {
		var photo Photo
		if err := rows.Scan(&photo.ID, &photo.AlbumID, &photo.Caption, &photo.UploaderPlayerID, &photo.UploaderName, &photo.UploaderRole, &photo.ContentType, &photo.Width, &photo.Height, &photo.ByteSize, &photo.CreatedAt); err != nil {
			return nil, err
		}
		photos = append(photos, photo)
	}
	return photos, rows.Err()
}

func (s *Service) StorePhoto(teamID, albumID string, actor Actor, caption string, source io.Reader) (Photo, error) {
	return s.storePhoto(teamID, albumID, actor, caption, source, uuid.NewString())
}

// A review ID gives callback retries a stable photo identity.
func (s *Service) StoreReviewedPhoto(teamID, albumID string, actor Actor, caption string, source io.Reader, reviewID string) (Photo, error) {
	if existing, err := s.photo(teamID, reviewID); err == nil {
		return existing, nil
	}
	return s.storePhoto(teamID, albumID, actor, caption, source, reviewID)
}
func (s *Service) storePhoto(teamID, albumID string, actor Actor, caption string, source io.Reader, id string) (Photo, error) {
	if err := validActor(actor); err != nil {
		return Photo{}, err
	}
	if _, err := s.album(teamID, albumID); err != nil {
		return Photo{}, err
	}
	data, err := io.ReadAll(io.LimitReader(source, MaxUploadBytes+1))
	if err != nil {
		return Photo{}, err
	}
	if len(data) == 0 || len(data) > MaxUploadBytes {
		return Photo{}, ErrInvalidPhoto
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !validFormat(format) || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > MaxPixels {
		return Photo{}, ErrInvalidPhoto
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || !validFormat(format) {
		return Photo{}, ErrInvalidPhoto
	}
	decoded = flatten(decoded)
	full := resize(decoded, FullImageEdge)
	thumbnail := resize(decoded, ThumbnailEdge)
	fullName := id + ".jpg"
	thumbName := id + "-thumb.jpg"
	fullPath := filepath.Join(s.root, "photos", fullName)
	thumbPath := filepath.Join(s.root, "thumbs", thumbName)
	if err := writeJPEGAtomically(fullPath, full, 85); err != nil {
		return Photo{}, err
	}
	if err := writeJPEGAtomically(thumbPath, thumbnail, 78); err != nil {
		_ = os.Remove(fullPath)
		return Photo{}, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		_ = os.Remove(fullPath)
		_ = os.Remove(thumbPath)
		return Photo{}, err
	}
	thumbInfo, err := os.Stat(thumbPath)
	if err != nil {
		_ = os.Remove(fullPath)
		_ = os.Remove(thumbPath)
		return Photo{}, err
	}
	if err := s.ensureTeamCapacity(teamID, info.Size()+thumbInfo.Size()); err != nil {
		_ = os.Remove(fullPath)
		_ = os.Remove(thumbPath)
		return Photo{}, err
	}
	now := nowISO()
	caption = strings.TrimSpace(caption)
	var uploaderPlayerID any
	if actor.Role == "player" {
		uploaderPlayerID = actor.PlayerID
	}
	_, err = s.db.Exec(`INSERT INTO photos (id, album_id, uploader_player_id, uploader_role, caption, file_name, thumbnail_file_name, content_type, width, height, byte_size, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, 'image/jpeg', ?, ?, ?, ?)`, id, albumID, uploaderPlayerID, actor.Role, caption, fullName, thumbName, full.Bounds().Dx(), full.Bounds().Dy(), info.Size()+thumbInfo.Size(), now)
	if err != nil {
		_ = os.Remove(fullPath)
		_ = os.Remove(thumbPath)
		return Photo{}, err
	}
	if _, err := s.db.Exec(`UPDATE albums SET updated_at = ? WHERE id = ?`, now, albumID); err != nil {
		return Photo{}, err
	}
	return s.photo(teamID, id)
}

func (s *Service) UpdateCaption(teamID, photoID string, actor Actor, caption string) (Photo, error) {
	if err := validActor(actor); err != nil {
		return Photo{}, err
	}
	photo, uploaderID, err := s.photoOwner(teamID, photoID)
	if err != nil {
		return Photo{}, err
	}
	if actor.Role == "player" && uploaderID != actor.PlayerID {
		return Photo{}, ErrForbidden
	}
	caption = strings.TrimSpace(caption)
	_, err = s.db.Exec(`UPDATE photos SET caption = ? WHERE id = ?`, caption, photo.ID)
	if err != nil {
		return Photo{}, err
	}
	return s.photo(teamID, photoID)
}

func (s *Service) DeletePhoto(teamID, photoID string, actor Actor) error {
	if err := validActor(actor); err != nil {
		return err
	}
	photo, uploaderID, fullName, thumbName, err := s.photoFiles(teamID, photoID)
	if err != nil {
		return err
	}
	if actor.Role == "player" && uploaderID != actor.PlayerID {
		return ErrForbidden
	}
	now := nowISO()
	if _, err := s.db.Exec(`UPDATE photos SET deleted_at = ? WHERE id = ? AND deleted_at = ''`, now, photo.ID); err != nil {
		return err
	}
	for _, fileName := range []string{fullName, thumbName} {
		if err := removeIfExists(s.pathFor(fileName, fileName == thumbName)); err != nil {
			return err
		}
	}
	_, err = s.db.Exec(`DELETE FROM photos WHERE id = ?`, photo.ID)
	return err
}

func (s *Service) OpenPhoto(teamID, photoID string, thumbnail bool) (*os.File, string, error) {
	_, _, fullName, thumbName, err := s.photoFiles(teamID, photoID)
	if err != nil {
		return nil, "", err
	}
	fileName := fullName
	if thumbnail {
		fileName = thumbName
	}
	file, err := os.Open(s.pathFor(fileName, thumbnail))
	if err != nil {
		return nil, "", err
	}
	return file, "image/jpeg", nil
}

func (s *Service) album(teamID, id string) (Album, error) {
	var album Album
	err := s.db.QueryRow(`SELECT a.id, COALESCE(a.event_id, ''), COALESCE(a.special_activity_id, ''), CASE WHEN a.event_id IS NOT NULL THEN 'event' ELSE 'special_activity' END, COALESCE(e.title, special.name), (SELECT COUNT(*) FROM photos p WHERE p.album_id = a.id AND p.deleted_at = ''), a.created_at, a.updated_at FROM albums a LEFT JOIN events e ON e.id = a.event_id LEFT JOIN special_activities special ON special.id = a.special_activity_id WHERE a.id = ? AND a.team_id = ?`, id, teamID).Scan(&album.ID, &album.EventID, &album.SpecialActivityID, &album.SourceKind, &album.SourceTitle, &album.PhotoCount, &album.CreatedAt, &album.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Album{}, ErrInvalidAlbum
	}
	return album, err
}

func (s *Service) photo(teamID, id string) (Photo, error) {
	var photo Photo
	err := s.db.QueryRow(`SELECT p.id, p.album_id, p.caption, COALESCE(p.uploader_player_id, ''), COALESCE(players.name, CASE p.uploader_role WHEN 'coach' THEN '教练' ELSE '队员' END), p.uploader_role, p.content_type, p.width, p.height, p.byte_size, p.created_at FROM photos p JOIN albums a ON a.id = p.album_id LEFT JOIN players ON players.id = p.uploader_player_id WHERE p.id = ? AND a.team_id = ? AND p.deleted_at = ''`, id, teamID).Scan(&photo.ID, &photo.AlbumID, &photo.Caption, &photo.UploaderPlayerID, &photo.UploaderName, &photo.UploaderRole, &photo.ContentType, &photo.Width, &photo.Height, &photo.ByteSize, &photo.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Photo{}, ErrInvalidPhoto
	}
	return photo, err
}

func (s *Service) photoOwner(teamID, id string) (Photo, string, error) {
	photo, uploaderID, _, _, err := s.photoFiles(teamID, id)
	return photo, uploaderID, err
}

func (s *Service) photoFiles(teamID, id string) (Photo, string, string, string, error) {
	var photo Photo
	var uploaderID, fullName, thumbName string
	err := s.db.QueryRow(`SELECT p.id, p.album_id, p.caption, COALESCE(p.uploader_player_id, ''), COALESCE(players.name, CASE p.uploader_role WHEN 'coach' THEN '教练' ELSE '队员' END), p.uploader_role, p.content_type, p.width, p.height, p.byte_size, p.created_at, COALESCE(p.uploader_player_id, ''), p.file_name, p.thumbnail_file_name
		FROM photos p JOIN albums a ON a.id = p.album_id LEFT JOIN players ON players.id = p.uploader_player_id
		WHERE p.id = ? AND a.team_id = ?`, id, teamID).Scan(&photo.ID, &photo.AlbumID, &photo.Caption, &photo.UploaderPlayerID, &photo.UploaderName, &photo.UploaderRole, &photo.ContentType, &photo.Width, &photo.Height, &photo.ByteSize, &photo.CreatedAt, &uploaderID, &fullName, &thumbName)
	if errors.Is(err, sql.ErrNoRows) {
		return Photo{}, "", "", "", ErrInvalidPhoto
	}
	if err != nil {
		return Photo{}, "", "", "", err
	}
	return photo, uploaderID, fullName, thumbName, nil
}

func validActor(actor Actor) error {
	if actor.Role == "coach" {
		return nil
	}
	if actor.Role == "player" && actor.PlayerID != "" {
		return nil
	}
	return ErrForbidden
}

func validFormat(format string) bool { return format == "jpeg" || format == "png" || format == "webp" }

func (s *Service) ensureTeamCapacity(teamID string, addition int64) error {
	var used int64
	err := s.db.QueryRow(`SELECT COALESCE(SUM(p.byte_size), 0) FROM photos p JOIN albums a ON a.id = p.album_id WHERE a.team_id = ? AND p.deleted_at = ''`, teamID).Scan(&used)
	if err != nil {
		return err
	}
	if addition < 0 || used > MaxTeamBytes-addition {
		return ErrStorageFull
	}
	return nil
}

func flatten(source image.Image) image.Image {
	bounds := source.Bounds()
	destination := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	stdraw.Draw(destination, destination.Bounds(), image.NewUniform(color.White), image.Point{}, stdraw.Src)
	stdraw.Draw(destination, destination.Bounds(), source, bounds.Min, stdraw.Over)
	return destination
}

func resize(source image.Image, edge int) image.Image {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= edge && height <= edge {
		return source
	}
	if width >= height {
		height = height * edge / width
		width = edge
	} else {
		width = width * edge / height
		height = edge
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	destination := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(destination, destination.Bounds(), source, bounds, draw.Over, nil)
	return destination
}

func writeJPEGAtomically(target string, source image.Image, quality int) error {
	temp := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+"."+uuid.NewString()+".tmp")
	file, err := os.OpenFile(temp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(file, source, &jpeg.Options{Quality: quality}); err != nil {
		_ = file.Close()
		_ = os.Remove(temp)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temp)
		return err
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}

func (s *Service) pathFor(fileName string, thumbnail bool) string {
	directory := "photos"
	if thumbnail {
		directory = "thumbs"
	}
	return filepath.Join(s.root, directory, filepath.Base(fileName))
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func nowISO() string { return time.Now().Format(time.RFC3339) }
