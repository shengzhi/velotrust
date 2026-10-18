package database

import (
	"time"

	"velotrust/shared/constants"
)

type ModelState = constants.ModelState

const (
	ModelState_None        = constants.ModelStateNone
	ModelState_New         = constants.ModelStateNew
	ModelState_Modified    = constants.ModelStateModified
	ModelState_SoftDeleted = constants.ModelStateSoftDeleted
	ModelState_HardDeleted = constants.ModelStateHardDeleted
)

type IModel interface {
	SetModelState(ModelState)

	State() ModelState

	UpdateCols() []string

	PKValues() []any

	Version() int
}

// 基础模型
type BaseModel struct {
	VersionNo  int        `gorm:"column:version_no" xorm:"'version_no' notnull comment('版本号')" json:"-"`
	CreatedBy  string     `gorm:"column:created_by;size:45" xorm:"'created_by' notnull comment('创建人')" json:"created_by"`
	CreatedAt  time.Time  `gorm:"column:created_at" xorm:"'created_at' notnull comment('创建时间')" json:"created_at"`
	UpdatedBy  string     `gorm:"column:updated_by;size:45" xorm:"'updated_by' notnull comment('修改人')" json:"updated_by"`
	UpdatedAt  time.Time  `gorm:"column:updated_at" xorm:"'updated_at' notnull comment('修改时间')" json:"updated_at"`
	state      ModelState `gorm:"-" xorm:"-"`
	updateCols []string   `gorm:"-" xorm:"-"`
}

// 可删除模型
type DeletableModel struct {
	BaseModel `xorm:"extends"`
	IsDeleted bool `gorm:"column:is_del" xorm:"is_del notnull comment('是否删除')" json:"is_del"`
}

// Version returns model.VersionNo - 1.
func (mb *BaseModel) Version() int { return mb.VersionNo - 1 }

func (mb *BaseModel) PKValues() []any { return []any{} }

func (mb *BaseModel) SetModelState(state ModelState) { mb.state = state }

func (mb *BaseModel) State() ModelState { return mb.state }

func (mb *BaseModel) CreateNew(name string) {
	if mb.state == ModelState_New {
		return
	}
	mb.UpdatedBy = name
	mb.CreatedBy = name
	mb.VersionNo = 1
	mb.CreatedAt = time.Now()
	mb.UpdatedAt = mb.CreatedAt
	mb.state = ModelState_New
}

func (mb *BaseModel) Update(name string) {
	if mb.state == ModelState_Modified {
		return
	}
	mb.VersionNo += 1
	mb.UpdatedAt = time.Now()
	mb.UpdatedBy = name
	mb.state = ModelState_Modified
}

func (mb *BaseModel) Cols(colNames ...string) {
	mb.updateCols = append(mb.updateCols, colNames...)
}

func (mb *BaseModel) UpdateCols() []string {
	if len(mb.updateCols) <= 0 {
		return []string{}
	}
	return append([]string{"version_no", "updated_by", "updated_at"}, mb.updateCols...)
}

func (mb *DeletableModel) SoftDelete(name string) {
	if mb.state == ModelState_SoftDeleted {
		return
	}
	mb.Cols("is_del")
	mb.VersionNo += 1
	mb.IsDeleted = true
	mb.UpdatedAt = time.Now()
	mb.UpdatedBy = name

	mb.state = ModelState_SoftDeleted
}

func (mb *DeletableModel) HardDelete() {
	if mb.state == ModelState_HardDeleted {
		return
	}
	mb.VersionNo += 1
	mb.state = ModelState_HardDeleted
}
