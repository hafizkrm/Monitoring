package devices

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hafizkrm/Monitoring/backend/internal/database"
)

func TestDeleteDeviceHandler(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock db: %v", err)
	}
	defer db.Close()

	testDB := &database.Database{DB: db}

	// 1. Transaction starts
	mock.ExpectBegin()

	// 2. Select IDs
	rows := sqlmock.NewRows([]string{"id"}).AddRow("1")
	mock.ExpectQuery("SELECT id FROM devices WHERE ip_address = \\?").
		WithArgs("192.168.1.1").
		WillReturnRows(rows)

	// 3. Delete from devices
	mock.ExpectExec("DELETE FROM devices WHERE ip_address = \\?").
		WithArgs("192.168.1.1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 4. Delete related (device_metrics, interface_metrics, polling_logs, incidents)
	mock.ExpectExec("DELETE FROM device_metrics").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM interface_metrics").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM polling_logs").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM incidents").WillReturnResult(sqlmock.NewResult(1, 1))

	// 5. Delete alerts and incidents
	mock.ExpectExec("DELETE FROM alerts").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM incidents").WillReturnResult(sqlmock.NewResult(1, 1))
	
	// 6. Delete fallback device_metrics
	mock.ExpectExec("DELETE FROM device_metrics").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM interface_metrics").WillReturnResult(sqlmock.NewResult(1, 1))

	// 7. Commit
	mock.ExpectCommit()

	// 8. Insert Activity Log
	mock.ExpectExec("INSERT INTO activity_logs").
		WillReturnResult(sqlmock.NewResult(1, 1))

	reqBody := DeleteDeviceReq{
		IP: "192.168.1.1",
	}
	b, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/devices/delete", bytes.NewReader(b))
	rr := httptest.NewRecorder()

	handler := DeleteDeviceHandler(testDB)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}
