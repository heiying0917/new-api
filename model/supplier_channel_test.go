package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func setupSupplierChannelTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
}

func TestChannelSupplierFields(t *testing.T) {
	setupSupplierChannelTables(t)
	cp := 2.5
	ch := &Channel{Name: "c1", Key: "k1", SupplierId: 7, CostPrice: &cp, Models: "gpt-4", Group: "default"}
	require.NoError(t, DB.Create(ch).Error)
	var got Channel
	require.NoError(t, DB.First(&got, "id = ?", ch.Id).Error)
	require.Equal(t, 7, got.SupplierId)
	require.NotNil(t, got.CostPrice)
	require.Equal(t, 2.5, *got.CostPrice)
}

func TestGetChannelsBySupplier(t *testing.T) {
	setupSupplierChannelTables(t)
	cp := 2.0
	require.NoError(t, DB.Create(&Channel{Name: "a", Key: "k1", SupplierId: 7, CostPrice: &cp, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "b", Key: "k2", SupplierId: 7, CostPrice: &cp, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "c", Key: "k3", SupplierId: 9, CostPrice: &cp, Models: "m", Group: "g"}).Error)

	list, total, err := GetChannelsBySupplier(7, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, list, 2)
	for _, c := range list {
		require.Equal(t, 7, c.SupplierId)
		require.Equal(t, "", c.Key)
	}
}

func TestSearchChannelsBySupplier(t *testing.T) {
	setupSupplierChannelTables(t)
	cp := 2.0
	require.NoError(t, DB.Create(&Channel{Name: "alpha", Key: "k1", SupplierId: 7, CostPrice: &cp, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "beta", Key: "k2", SupplierId: 7, CostPrice: &cp, Models: "m", Group: "g"}).Error)
	list, total, err := SearchChannelsBySupplier(7, "alpha", 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	require.Equal(t, "alpha", list[0].Name)
}
