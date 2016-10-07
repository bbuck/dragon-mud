package models

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/bbuck/dragon-mud/data"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/utils"
	"github.com/satori/go.uuid"
)

type BeforeSaver interface {
	BeforeSave() error
}

type AfterSaver interface {
	AfterSave() error
}

type BeforeCreater interface {
	BeforeCreate() error
}

type AfterCreater interface {
	AfterCreate() error
}

type BeforeUpdater interface {
	BeforeCreate() error
}

type AfterUpdater interface {
	AfterUpdate() error
}

type BeforeDeleter interface {
	BeforeDelete() error
}

type AfterDeleter interface {
	AfterDelete() error
}

type SoftDeleter interface {
	SoftDelete()
}

type Model interface {
	UUID() string
	AssignUUID()
	IsNewRecord() bool
	DB() data.DB
}

// BaseModel contains the components every model should have.
type BaseModel struct {
	UUID      string    `json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UUID returns the UUID field on the BaseModel to match the Model interface.
func (b *BaseModel) UUID() string {
	return b.UUID
}

// AssignUUID will create a new UUID and assign it to this model, only if it's
// a new record.
func (b *BaseModel) AssignUUID() {
	if b.IsNewRecord() {
		b.UUID = uuid.NewV4().String()
	}
}

// DB is shorthand for fetching a reference to the database handle. All models
// should embed BaseModel and therefore benefit from this method.
func (*BaseModel) DB() data.DB {
	return data.DefaultFactory.MustOpen()
}

// IsNewRecord checks for a UUID field set on the object. UUIDs are set when
// an object is created.
func (b BaseModel) IsNewRecord() bool {
	return len(b.UUID) == 0
}

// SoftDeletedModel defines fields necessary to make a model delete "softly"
// (or in other words, not delete but mark itself deleted).
type SoftDeletedModel struct {
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// SoftDelete matches the SoftDeleter interaface which assigns a models deleted
// at property.
func (sdm *SoftDeletedModel) SoftDelete() {
	sdm.DeletedAt = time.Now()
}

// ScriptableModel contains the concerns for a model that should have a script
// applied and tracked on it.
type ScriptableModel struct {
	Script          string    `json:"script" sql:"type:text"`
	ScriptUpdatedAt time.Time `json:"script_updated_at"`
}

// Save will persist a model in the database. If the model is a new record then
// the record is created otherwose it's updated.
func Save(model Model) {
	if err := beforeSave(model); err != nil {
		return
	}
	db := data.DefaultFactory.MustOpen()
	if model.IsNewRecord() {
		model.AssignUUID()
		if err := beforeCreate(model); err != nil {
			return
		}
		db.Create(model)
		afterCreate(model)
	} else {
		if err := beforeUpdate(model); err != nil {
			return
		}
		db.Save(model)
		afterUpdate(model)
	}
	afterSave(model)
}

// Delete will remove a record from persistence in the database. The database
// manager will automatically handle soft deletes if the model defines a
// "DeletedAt" field.
func Delete(model Model) {
	if err := beforeDelete(model); err != nil {
		return
	}
	db := data.DefaultFactory.MustOpen()
	db.Delete(model)
	afterDelete(model)
}

// --- callback helpers

func beforeSave(model Model) error {
	if bs, ok := model.(BeforeSaver); ok {
		if err := bs.BeforeSave(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
				"model": utils.ToJSON(record),
			}).Error("BeforeSave failed for model.")

			return err
		}
	}

	return nil
}

func afterSave(model Model) error {
	if bs, ok := model.(AfterSaver); ok {
		if err := bs.AfterSave(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
				"model": utils.ToJSON(record),
			}).Error("AfterSave failed for model.")

			return err
		}
	}

	return nil
}

func beforeCreate(model Model) error {
	if bs, ok := model.(BeforeCreater); ok {
		if err := bs.BeforeCreate(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
				"model": utils.ToJSON(record),
			}).Error("BeforeCreate failed for model.")

			return err
		}
	}

	return nil
}

func afterCreate(model Model) error {
	if bs, ok := model.(AfterCreater); ok {
		if err := bs.AfterCreate(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
				"model": utils.ToJSON(record),
			}).Error("AfterCreate failed for model.")

			return err
		}
	}

	return nil
}

func beforeUpdate(model Model) error {
	if bs, ok := model.(BeforeUpdater); ok {
		if err := bs.BeforeUpate(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
				"model": utils.ToJSON(record),
			}).Error("BeforeUpdate failed for model.")

			return err
		}
	}

	return nil
}

func afterUpdate(model Model) error {
	if bs, ok := model.(AfterUpdater); ok {
		if err := bs.AfterUpdate(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
				"model": utils.ToJSON(record),
			}).Error("AfterUpdate failed for model.")

			return err
		}
	}

	return nil
}

func beforeDelete(model Model) error {
	if bs, ok := model.(BeforeDeleter); ok {
		if err := bs.BeforeDelete(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
				"model": utils.ToJSON(record),
			}).Error("BeforeDelete failed for model.")

			return err
		}
	}

	return nil
}

func afterDelete(model Model) error {
	if bs, ok := model.(AfterDeleter); ok {
		if err := bs.AfterDelete(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
				"model": utils.ToJSON(record),
			}).Error("AfterDelete failed for model.")

			return err
		}
	}

	return nil
}
