package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"todo/internal/todo"
)

// attachLabels loads the labels of items (all of the user's items, or only
// item onlyID if it is not 0) and puts them on the items, sorted by name.
func attachLabels(ctx context.Context, db execer, userID int64, items []todo.Item, onlyID int64) error {
	if len(items) == 0 {
		return nil
	}
	q := `SELECT il.item_id, l.id, l.name, l.color
	      FROM item_labels il JOIN labels l ON l.id = il.label_id
	      WHERE l.user_id = ?`
	args := []any{userID}
	if onlyID != 0 {
		q += ` AND il.item_id = ?`
		args = append(args, onlyID)
	}
	q += ` ORDER BY l.name, l.id`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("read labels: %w", err)
	}
	defer rows.Close()
	byItem := map[int64][]todo.Label{}
	for rows.Next() {
		var itemID int64
		l := todo.Label{UserID: userID}
		if err := rows.Scan(&itemID, &l.ID, &l.Name, &l.Color); err != nil {
			return fmt.Errorf("read labels: %w", err)
		}
		byItem[itemID] = append(byItem[itemID], l)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read labels: %w", err)
	}
	for i := range items {
		items[i].Labels = byItem[items[i].ID]
	}
	return nil
}

// setItemLabels links item id to the labels named names, creating the ones
// the user does not have yet. It returns todo.ErrTooManyLabels if the user
// would get more than todo.MaxLabelsPerUser labels.
func setItemLabels(ctx context.Context, tx *sql.Tx, userID, itemID int64, names []string, now time.Time) error {
	if len(names) == 0 {
		return nil
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM labels WHERE user_id = ?`, userID).Scan(&count); err != nil {
		return fmt.Errorf("save labels: %w", err)
	}
	for _, name := range names {
		var labelID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM labels WHERE user_id = ? AND name = ?`, userID, name).Scan(&labelID)
		if errors.Is(err, sql.ErrNoRows) {
			if count >= todo.MaxLabelsPerUser {
				return todo.ErrTooManyLabels
			}
			color := todo.LabelColors[count%len(todo.LabelColors)]
			res, err := tx.ExecContext(ctx,
				`INSERT INTO labels (user_id, name, color, created_at) VALUES (?, ?, ?, ?)`,
				userID, name, color, formatTime(now))
			if err != nil {
				return fmt.Errorf("save labels: %w", err)
			}
			if labelID, err = res.LastInsertId(); err != nil {
				return fmt.Errorf("save labels: %w", err)
			}
			count++
		} else if err != nil {
			return fmt.Errorf("save labels: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO item_labels (item_id, label_id) VALUES (?, ?)`, itemID, labelID); err != nil {
			return fmt.Errorf("save labels: %w", err)
		}
	}
	return nil
}

func (s *Store) Labels(ctx context.Context, userID int64) ([]todo.Label, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT l.id, l.name, l.color, COUNT(il.item_id)
		 FROM labels l LEFT JOIN item_labels il ON il.label_id = l.id
		 WHERE l.user_id = ?
		 GROUP BY l.id ORDER BY l.name, l.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	defer rows.Close()
	labels := []todo.Label{}
	for rows.Next() {
		l := todo.Label{UserID: userID}
		if err := rows.Scan(&l.ID, &l.Name, &l.Color, &l.Items); err != nil {
			return nil, fmt.Errorf("list labels: %w", err)
		}
		labels = append(labels, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	return labels, nil
}

func (s *Store) Label(ctx context.Context, userID, id int64) (todo.Label, error) {
	l := todo.Label{UserID: userID}
	err := s.db.QueryRowContext(ctx,
		`SELECT l.id, l.name, l.color, (SELECT COUNT(*) FROM item_labels WHERE label_id = l.id)
		 FROM labels l WHERE l.id = ? AND l.user_id = ?`, id, userID).Scan(&l.ID, &l.Name, &l.Color, &l.Items)
	if errors.Is(err, sql.ErrNoRows) {
		return todo.Label{}, todo.ErrNotFound
	}
	if err != nil {
		return todo.Label{}, fmt.Errorf("read label: %w", err)
	}
	return l, nil
}

func (s *Store) SetLabelColor(ctx context.Context, userID, id int64, color string) error {
	return s.update(ctx, `UPDATE labels SET color = ? WHERE id = ? AND user_id = ?`, color, id, userID)
}

func (s *Store) DeleteLabel(ctx context.Context, userID, id int64) error {
	return s.update(ctx, `DELETE FROM labels WHERE id = ? AND user_id = ?`, id, userID)
}

// AddItemLabel puts label name on an item in one transaction, with the
// 5-per-item check inside it (code review 3).
func (s *Store) AddItemLabel(ctx context.Context, userID, itemID int64, name string, now time.Time) (todo.Item, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return todo.Item{}, fmt.Errorf("add label: %w", err)
	}
	defer tx.Rollback()
	if err := touchItem(ctx, tx, userID, itemID, now); err != nil {
		return todo.Item{}, err
	}
	var has, count int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(l.name = ?), 0), COUNT(*) FROM item_labels il JOIN labels l ON l.id = il.label_id WHERE il.item_id = ?`,
		name, itemID).Scan(&has, &count); err != nil {
		return todo.Item{}, fmt.Errorf("add label: %w", err)
	}
	if has > 0 {
		// Nothing to change. End the transaction first: it holds the only
		// connection, and Get needs it.
		tx.Rollback()
		return s.Get(ctx, userID, itemID)
	}
	if count >= todo.MaxLabelsPerItem {
		return todo.Item{}, todo.ErrItemLabelsFull
	}
	if err := setItemLabels(ctx, tx, userID, itemID, []string{name}, now); err != nil {
		return todo.Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return todo.Item{}, fmt.Errorf("add label: %w", err)
	}
	return s.Get(ctx, userID, itemID)
}

// RemoveItemLabel takes one label off an item in one transaction.
func (s *Store) RemoveItemLabel(ctx context.Context, userID, itemID, labelID int64, now time.Time) (todo.Item, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return todo.Item{}, fmt.Errorf("remove label: %w", err)
	}
	defer tx.Rollback()
	if err := touchItem(ctx, tx, userID, itemID, now); err != nil {
		return todo.Item{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_labels WHERE item_id = ? AND label_id = ?`, itemID, labelID); err != nil {
		return todo.Item{}, fmt.Errorf("remove label: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return todo.Item{}, fmt.Errorf("remove label: %w", err)
	}
	return s.Get(ctx, userID, itemID)
}

// touchItem sets updated_at of the user's item, or returns todo.ErrNotFound.
func touchItem(ctx context.Context, tx *sql.Tx, userID, itemID int64, now time.Time) error {
	return updateOne(ctx, tx, `UPDATE items SET updated_at = ? WHERE id = ? AND user_id = ?`, formatTime(now), itemID, userID)
}
