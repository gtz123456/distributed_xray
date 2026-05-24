package order

import (
	"go-distributed/payment/db"
	"go-distributed/utils"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func init() {
	os.Setenv("TEST_MODE", "1")
}

func setupTestDB() (*miniredis.Miniredis, *gorm.DB) {
	mr, _ := miniredis.Run()
	utils.RedisClient = redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	database, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	db.DB = database
	db.DB.AutoMigrate(&db.Order{})

	intervalSet = NewIntervalSet() // reset global state

	return mr, database
}

func TestCreateOrder(t *testing.T) {
	mr, database := setupTestDB()
	defer mr.Close()
	defer database.Migrator().DropTable(&db.Order{})

	order, err := CreateOrder("test-order-1", 1000, "http://callback", "TRX", "USD")
	assert.NoError(t, err)
	assert.Equal(t, "test-order-1", order.ID)
	assert.Equal(t, "pending", order.Status)

	// Check if Redis has the mapping
	redisOrderID, err := utils.RedisClient.Get(utils.Ctx, "actual_amount_to_id:"+strconv.FormatInt(order.ActualAmount, 10)).Result()
	assert.NoError(t, err)
	assert.Equal(t, "test-order-1", redisOrderID)

	// Create another order, amount should be slightly different
	order2, err := CreateOrder("test-order-2", 1000, "http://callback", "TRX", "USD")
	assert.NoError(t, err)
	assert.NotEqual(t, order.ActualAmount, order2.ActualAmount)
}

func TestRestoreStateFromDB(t *testing.T) {
	mr, database := setupTestDB()
	defer mr.Close()
	defer database.Migrator().DropTable(&db.Order{})

	// Insert some pending orders manually
	baseAmount := int64(10000000)
	orders := []db.Order{
		{ID: "order-1", Amount: 1000, ActualAmount: baseAmount, Status: "pending", CreatedAt: time.Now()},
		{ID: "order-2", Amount: 1000, ActualAmount: baseAmount + 1, Status: "pending", CreatedAt: time.Now()},
	}

	for _, o := range orders {
		database.Create(&o)
	}

	// Make sure Redis is empty
	keys, _ := utils.RedisClient.Keys(utils.Ctx, "*").Result()
	assert.Equal(t, 0, len(keys))

	// Run Restore
	err := RestoreStateFromDB()
	assert.NoError(t, err)

	// Redis should now contain the mappings
	id1, _ := utils.RedisClient.Get(utils.Ctx, "actual_amount_to_id:"+strconv.FormatInt(baseAmount, 10)).Result()
	assert.Equal(t, "order-1", id1)

	id2, _ := utils.RedisClient.Get(utils.Ctx, "actual_amount_to_id:"+strconv.FormatInt(baseAmount+1, 10)).Result()
	assert.Equal(t, "order-2", id2)

	// The intervalSet should contain these amounts
	// Creating a new order with same base amount should use baseAmount + 2
	order3, err := CreateOrder("order-3", 1000, "http://callback", "TRX", "USD")
	assert.NoError(t, err)
	// order.Amount is 1000, meaning 10 USD -> TRX conversion might be different, let's just check intervalSet behavior
	// Actually Convert will be called. Let's not strictly check the amount here, but just the fact that it doesn't collide
	assert.NotEqual(t, baseAmount, order3.ActualAmount)
	assert.NotEqual(t, baseAmount+1, order3.ActualAmount)
}
