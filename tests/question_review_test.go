package tests

import (
	"errors"
	"testing"

	"form_pf_backend/db"
	"form_pf_backend/models"

	"github.com/google/uuid"
)

func TestQuestionReviewTableName(t *testing.T) {
	if (models.QuestionReview{}).TableName() != "question_reviews" {
		t.Fatalf("table = %s", (models.QuestionReview{}).TableName())
	}
}

func TestQuestionReviewRoundTrip(t *testing.T) {
	loadBackendEnv(t)

	filingID := uuid.NewString()
	fundID := "review-test-fund"
	fieldNo := "5"
	userID := uuid.NewString()
	otherUser := uuid.NewString()

	t.Cleanup(func() {
		_ = db.DeleteQuestionReview(filingID, fundID, fieldNo, userID)
		_ = db.DeleteQuestionReview(filingID, fundID, fieldNo, otherUser)
	})

	created, err := db.CreateQuestionReview(filingID, fundID, fieldNo, userID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !created {
		t.Fatal("first review should insert a row")
	}

	created, err = db.CreateQuestionReview(filingID, fundID, fieldNo, userID)
	if err != nil {
		t.Fatalf("duplicate create: %v", err)
	}
	if created {
		t.Fatal("second review by the same user should not insert another row")
	}

	if _, err := db.CreateQuestionReview(filingID, fundID, fieldNo, otherUser); err != nil {
		t.Fatalf("second reviewer: %v", err)
	}

	rows, err := db.ListQuestionReviews(filingID, fundID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("reviewers = %d, want 2", len(rows))
	}

	if err := db.DeleteQuestionReview(filingID, fundID, fieldNo, userID); err != nil {
		t.Fatalf("delete own review: %v", err)
	}
	rows, err = db.ListQuestionReviews(filingID, fundID)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(rows) != 1 || rows[0].CreatedBy != otherUser {
		t.Fatalf("remaining reviews = %+v", rows)
	}

	err = db.DeleteQuestionReview(filingID, fundID, fieldNo, userID)
	if !errors.Is(err, db.ErrReviewNotFound) {
		t.Fatalf("second undo err = %v, want ErrReviewNotFound", err)
	}
}

func TestQuestionFieldExists(t *testing.T) {
	loadBackendEnv(t)

	ok, err := db.QuestionFieldExists("5")
	if err != nil {
		t.Fatalf("field 5: %v", err)
	}
	if !ok {
		t.Fatal("field 5 should exist in form_fields_ms_legacy")
	}

	ok, err = db.QuestionFieldExists("not-a-real-question")
	if err != nil {
		t.Fatalf("missing field: %v", err)
	}
	if ok {
		t.Fatal("unknown field should not exist")
	}
}
