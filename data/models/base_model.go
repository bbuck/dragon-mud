package models

import (
	"time"

	"github.com/bbuck/dragon-mud/data"
	"github.com/jinzhu/gorm"
)

// BaseModel contains the components every model should have.
type BaseModel struct {
	ID        uint      `json:"id" gorm:"primary_key" sql:"AUTO_INCREMENT"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DB is shorthand for fetching a reference to the database handle. All models
// should embed BaseModel and therefore benefit from this method.
func (b BaseModel) DB() *gorm.DB {
	return data.DefaultFactory.MustOpen()
}

// SoftDeletedModel defines fields necessary to make a model delete "softly"
// (or in other words, not delete but mark itself deleted).
type SoftDeletedModel struct {
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// ScriptableModel contains the concerns for a model that should have a script
// applied and tracked on it.
type ScriptableModel struct {
	Script          string    `json:"script" sql:"type:text"`
	ScriptUpdatedAt time.Time `json:"script_updated_at"`
}

// IsNewRecord will return true if the record is new, or has been created before.
func IsNewRecord(model interface{}) bool {
	db := data.DefaultFactory.MustOpen()

	return db.NewRecord(model)
}

// Save will persist a model in the database. If the model is a new record then
// the record is created otherwose it's updated.
func Save(model interface{}) {
	db := data.DefaultFactory.MustOpen()
	if db.NewRecord(model) {
		db.Create(model)
	} else {
		db.Save(model)
	}
}

// Delete will remove a record from persistence in the database. The database
// manager will automatically handle soft deletes if the model defines a
// "DeletedAt" field.
func Delete(model interface{}) {
	db := data.DefaultFactory.MustOpen()
	db.Delete(model)
}

// ByID returns a gorm DB primed for search for a record by it's ID. This is
// shorthand for caching DB references and calling Where(id)
func ByID(id uint) *gorm.DB {
	db := data.DefaultFactory.MustOpen()
	return db.Where(id)
}
