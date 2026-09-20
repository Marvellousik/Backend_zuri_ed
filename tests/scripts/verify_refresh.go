package main

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DeviceInfo represents client device information for refresh token testing.
type DeviceInfo struct {
	DeviceName string `json:"device_name"`
	OS         string `json:"os"`
	Browser    string `json:"browser"`
}

func (d *DeviceInfo) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("failed to unmarshal JSONB value")
	}
	return json.Unmarshal(bytes, d)
}

func (d DeviceInfo) Value() (driver.Value, error) {
	return json.Marshal(d)
}

// RefreshToken represents a refresh token record.
type RefreshToken struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	UserID     uuid.UUID  `gorm:"type:uuid;not null;index"`
	TokenHash  string     `gorm:"type:varchar(255);not null;uniqueIndex"`
	DeviceInfo DeviceInfo `gorm:"type:jsonb"`
	IPAddress  string     `gorm:"type:varchar(45)"`
	IsRevoked  bool       `gorm:"default:false"`
	ExpiresAt  time.Time  `gorm:"not null"`
	CreatedAt  time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
}

func main() {
	dsn := "host=localhost user=zuri password=zuri_secret dbname=zuri port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Printf("FAILED: connect: %v\n", err)
		return
	}

	db.AutoMigrate(&RefreshToken{})

	create := &RefreshToken{
		TokenHash:  "testhash_" + fmt.Sprintf("%d", 1),
		DeviceInfo: DeviceInfo{DeviceName: "iPhone", OS: "iOS"},
		IPAddress:  "127.0.0.1",
	}
	if err := db.Create(create).Error; err != nil {
		fmt.Printf("FAILED: create: %v\n", err)
		return
	}
	fmt.Println("✓ Create succeeded")

	var read RefreshToken
	if err := db.First(&read, "token_hash = ?", create.TokenHash).Error; err != nil {
		fmt.Printf("FAILED: read: %v\n", err)
		return
	}
	fmt.Println("✓ Read succeeded")

	if read.DeviceInfo.DeviceName != "iPhone" {
		fmt.Printf("FAILED: data mismatch: %+v\n", read.DeviceInfo)
		return
	}
	fmt.Println("✓ Data verified")

	// Direct Scan test with string (simulates pgx)
	var di DeviceInfo
	if err := di.Scan(`{"device_name":"Pixel","os":"Android"}`); err != nil {
		fmt.Printf("FAILED: Scan(string): %v\n", err)
		return
	}
	fmt.Println("✓ Scan(string) succeeded")

	fmt.Println("\n=== ALL REFRESH TOKEN TESTS PASSED ===")
}
