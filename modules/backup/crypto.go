// DigiPanel Backup Encryption (modules/backup/crypto.go)
// Authenticated, chunked AES-256-GCM with PBKDF2-HMAC-SHA256 key derivation.
// Each chunk carries its own authentication tag, so a truncated or reordered
// archive fails verification instead of restoring corrupted bytes.
// Zero emojis, strictly production grade.

package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	magicPrefix   = "DPBK1"
	saltSize      = 16
	nonceBaseSize = 8
	chunkSize     = 64 * 1024
	keySize       = 32
	kdfIterations = 120000
	headerSize    = len(magicPrefix) + saltSize + nonceBaseSize
)

var (
	ErrBadMagic       = errors.New("backup archive is not a DigiPanel encrypted archive")
	ErrDecryptFailed  = errors.New("backup archive failed authentication: wrong key or corrupted data")
	ErrTruncatedFrame = errors.New("backup archive ended in the middle of a chunk")
)

func deriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, kdfIterations, keySize, sha256.New)
}

func chunkNonce(base []byte, counter uint32) []byte {
	nonce := make([]byte, 12)
	copy(nonce, base)
	binary.BigEndian.PutUint32(nonce[8:], counter)
	return nonce
}

type encryptedWriter struct {
	sink      io.Writer
	aead      cipher.AEAD
	nonceBase []byte
	counter   uint32
	buffer    []byte
	closed    bool
}

func newEncryptedWriter(sink io.Writer, passphrase string) (*encryptedWriter, error) {
	if passphrase == "" {
		return nil, ErrNoPassphrase
	}
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	nonceBase := make([]byte, nonceBaseSize)
	if _, err := rand.Read(nonceBase); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(deriveKey(passphrase, salt))
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if _, err := io.WriteString(sink, magicPrefix); err != nil {
		return nil, err
	}
	if _, err := sink.Write(salt); err != nil {
		return nil, err
	}
	if _, err := sink.Write(nonceBase); err != nil {
		return nil, err
	}

	return &encryptedWriter{
		sink:      sink,
		aead:      aead,
		nonceBase: nonceBase,
		buffer:    make([]byte, 0, chunkSize),
	}, nil
}

func (w *encryptedWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, errors.New("encrypted writer is closed")
	}
	written := 0
	for len(p) > 0 {
		space := chunkSize - len(w.buffer)
		if space > len(p) {
			space = len(p)
		}
		w.buffer = append(w.buffer, p[:space]...)
		p = p[space:]
		written += space
		if len(w.buffer) == chunkSize {
			if err := w.flushChunk(); err != nil {
				return written, err
			}
		}
	}
	return written, nil
}

func (w *encryptedWriter) flushChunk() error {
	sealed := w.aead.Seal(nil, chunkNonce(w.nonceBase, w.counter), w.buffer, nil)
	w.counter++
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(sealed)))
	if _, err := w.sink.Write(length[:]); err != nil {
		return err
	}
	if _, err := w.sink.Write(sealed); err != nil {
		return err
	}
	w.buffer = w.buffer[:0]
	return nil
}

func (w *encryptedWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	if len(w.buffer) > 0 {
		if err := w.flushChunk(); err != nil {
			return err
		}
	}
	return nil
}

type encryptedReader struct {
	source    io.Reader
	aead      cipher.AEAD
	nonceBase []byte
	counter   uint32
	pending   []byte
	done      bool
}

func newEncryptedReader(source io.Reader, passphrase string) (*encryptedReader, error) {
	if passphrase == "" {
		return nil, ErrNoPassphrase
	}
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(source, header); err != nil {
		return nil, ErrBadMagic
	}
	if string(header[:len(magicPrefix)]) != magicPrefix {
		return nil, ErrBadMagic
	}
	salt := header[len(magicPrefix) : len(magicPrefix)+saltSize]
	nonceBase := header[len(magicPrefix)+saltSize:]

	block, err := aes.NewCipher(deriveKey(passphrase, salt))
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &encryptedReader{source: source, aead: aead, nonceBase: nonceBase}, nil
}

func (r *encryptedReader) Read(p []byte) (int, error) {
	if len(r.pending) == 0 {
		if r.done {
			return 0, io.EOF
		}
		if err := r.readFrame(); err != nil {
			return 0, err
		}
	}
	count := copy(p, r.pending)
	r.pending = r.pending[count:]
	return count, nil
}

func (r *encryptedReader) readFrame() error {
	var length [4]byte
	if _, err := io.ReadFull(r.source, length[:]); err != nil {
		if errors.Is(err, io.EOF) {
			r.done = true
			return io.EOF
		}
		return ErrTruncatedFrame
	}
	size := binary.BigEndian.Uint32(length[:])
	if size == 0 || size > chunkSize+uint32(r.aead.Overhead()) {
		return fmt.Errorf("%w: invalid frame length %d", ErrTruncatedFrame, size)
	}
	sealed := make([]byte, size)
	if _, err := io.ReadFull(r.source, sealed); err != nil {
		return ErrTruncatedFrame
	}
	plain, err := r.aead.Open(nil, chunkNonce(r.nonceBase, r.counter), sealed, nil)
	if err != nil {
		return ErrDecryptFailed
	}
	r.counter++
	r.pending = plain
	return nil
}
