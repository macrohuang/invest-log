package investlog

import (
	"database/sql"
	"fmt"
	"strings"
)

// AddAccount inserts a new account.
func (c *Core) AddAccount(account Account) (bool, error) {
	if account.AccountID == "" || account.AccountName == "" {
		return false, fmt.Errorf("account_id and account_name are required")
	}
	_, err := c.db.Exec(`
		INSERT INTO accounts (account_id, account_name, broker, account_type)
		VALUES (?, ?, ?, ?)
	`, account.AccountID, account.AccountName, account.Broker, account.AccountType)
	if err != nil {
		return false, err
	}
	return true, nil
}

func ensureAccountTx(tx *sql.Tx, accountID string, accountName *string) error {
	name := ""
	if accountName != nil {
		name = strings.TrimSpace(*accountName)
	}
	if name == "" {
		name = accountID
	}
	_, err := tx.Exec(`
		INSERT OR IGNORE INTO accounts (account_id, account_name)
		VALUES (?, ?)
	`, accountID, name)
	return err
}

// GetAccounts returns all accounts.
func (c *Core) GetAccounts() ([]Account, error) {
	rows, err := c.db.Query("SELECT account_id, account_name, broker, account_type, created_at FROM accounts ORDER BY account_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var acc Account
		var broker, accType, createdAt sql.NullString
		if err := rows.Scan(&acc.AccountID, &acc.AccountName, &broker, &accType, &createdAt); err != nil {
			return nil, err
		}
		if broker.Valid {
			acc.Broker = &broker.String
		}
		if accType.Valid {
			acc.AccountType = &accType.String
		}
		if createdAt.Valid {
			acc.CreatedAt = &createdAt.String
		}
		accounts = append(accounts, acc)
	}
	return accounts, rows.Err()
}

// CheckAccountInUse returns true if the account has transactions.
func (c *Core) CheckAccountInUse(accountID string) (bool, error) {
	var count int
	if err := c.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE account_id = ?", accountID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteAccount deletes an account if unused.
func (c *Core) DeleteAccount(accountID string) (bool, string, error) {
	inUse, err := c.CheckAccountInUse(accountID)
	if err != nil {
		return false, "", err
	}
	if inUse {
		return false, "Cannot delete: transactions exist for this account", nil
	}
	result, err := c.db.Exec("DELETE FROM accounts WHERE account_id = ?", accountID)
	if err != nil {
		return false, "", err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, "", err
	}
	if rows > 0 {
		return true, "Account deleted", nil
	}
	return false, "Account not found", nil
}
