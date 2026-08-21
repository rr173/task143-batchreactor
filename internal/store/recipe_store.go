package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"task143-batchreactor/internal/model"
)

// --- Recipe ---

// CreateRecipe inserts a recipe row.
func (s *Store) CreateRecipe(ctx context.Context, tx *sql.Tx, r *model.Recipe) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO recipes(id,name,product,k0,ea,reaction_order,ca0,delta_h,rho,cp,t0,jacket_temp,thermal_limit,min_conversion,duration,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Name, r.Product, r.K0, r.Ea, int(r.Order), r.CA0, r.DeltaHrx, r.Rho, r.Cp, r.T0, r.JacketTemp, r.ThermalLimit, r.MinConversion, r.Duration, r.CreatedAt)
	if err != nil {
		return fmt.Errorf("create recipe: %w", err)
	}
	return nil
}

// ListRecipes returns all recipes.
func (s *Store) ListRecipes(ctx context.Context) ([]model.Recipe, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,product,k0,ea,reaction_order,ca0,delta_h,rho,cp,t0,jacket_temp,thermal_limit,min_conversion,duration,created_at FROM recipes ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list recipes: %w", err)
	}
	defer rows.Close()
	var out []model.Recipe
	for rows.Next() {
		r, err := scanRecipe(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRecipe returns one recipe.
func (s *Store) GetRecipe(ctx context.Context, id string) (*model.Recipe, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,product,k0,ea,reaction_order,ca0,delta_h,rho,cp,t0,jacket_temp,thermal_limit,min_conversion,duration,created_at FROM recipes WHERE id=?`, id)
	return scanRecipeRow(row)
}

// GetRecipeTx returns one recipe inside a transaction.
func (s *Store) GetRecipeTx(ctx context.Context, tx *sql.Tx, id string) (*model.Recipe, error) {
	row := tx.QueryRowContext(ctx, `SELECT id,name,product,k0,ea,reaction_order,ca0,delta_h,rho,cp,t0,jacket_temp,thermal_limit,min_conversion,duration,created_at FROM recipes WHERE id=?`, id)
	return scanRecipeRow(row)
}

type recipeScanner interface {
	Scan(dest ...any) error
}

func scanRecipeRow(row recipeScanner) (*model.Recipe, error) {
	r, err := scanRecipe(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func scanRecipe(sc recipeScanner) (model.Recipe, error) {
	var r model.Recipe
	var order int
	if err := sc.Scan(&r.ID, &r.Name, &r.Product, &r.K0, &r.Ea, &order, &r.CA0, &r.DeltaHrx, &r.Rho, &r.Cp, &r.T0, &r.JacketTemp, &r.ThermalLimit, &r.MinConversion, &r.Duration, &r.CreatedAt); err != nil {
		return r, err
	}
	r.Order = model.ReactionOrder(order)
	return r, nil
}
