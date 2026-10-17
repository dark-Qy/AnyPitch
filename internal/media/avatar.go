package media

import (
	"bytes"
	"database/sql"
	"errors"
	"image"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

var ErrAvatarNotFound = errors.New("avatar not found")

// StoreAvatar keeps one small, server-owned JPEG per WeChat member.
func (s *Service) StoreAvatar(teamID, playerID string, source io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(source, MaxUploadBytes+1))
	if err != nil {
		return err
	}
	if len(data) == 0 || len(data) > MaxUploadBytes {
		return ErrInvalidPhoto
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !validFormat(format) || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > MaxPixels {
		return ErrInvalidPhoto
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || !validFormat(format) {
		return ErrInvalidPhoto
	}
	directory := filepath.Join(s.root, "avatars")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	name := uuid.NewString() + ".jpg"
	path := filepath.Join(directory, name)
	if err := writeJPEGAtomically(path, resize(flatten(decoded), 320), 82); err != nil {
		return err
	}
	var oldName string
	err = s.db.QueryRow(`SELECT m.avatar_file_name FROM wechat_members m JOIN players p ON p.id=m.player_id WHERE p.id=? AND p.team_id=? AND p.status='active'`, playerID, teamID).Scan(&oldName)
	if err != nil {
		_ = os.Remove(path)
		return err
	}
	if _, err := s.db.Exec(`UPDATE wechat_members SET avatar_file_name=? WHERE player_id=?`, name, playerID); err != nil {
		_ = os.Remove(path)
		return err
	}
	if oldName != "" {
		_ = os.Remove(filepath.Join(directory, filepath.Base(oldName)))
	}
	return nil
}

func (s *Service) AvatarPath(teamID, playerID string) (string, error) {
	var name string
	err := s.db.QueryRow(`SELECT m.avatar_file_name FROM wechat_members m JOIN players p ON p.id=m.player_id WHERE p.id=? AND p.team_id=? AND p.status='active'`, playerID, teamID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrAvatarNotFound
	}
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", ErrAvatarNotFound
	}
	return filepath.Join(s.root, "avatars", filepath.Base(name)), nil
}
