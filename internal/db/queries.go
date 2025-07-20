package db

const (
	QueryCreateUser = `
        INSERT INTO users (login, password)
        VALUES ($1, $2)
        RETURNING id, login, created_at
    `

	QueryGetUserByLogin = `
		SELECT id, login, password, created_at
		FROM users
		WHERE login = $1
	`

	QueryCreateAd = `
    INSERT INTO ads (title, text, image_url, price, user_id)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING id, title, text, image_url, price, user_id, created_at,
              (SELECT login FROM users WHERE id = $5) AS login,
              CASE WHEN user_id = $5 THEN true ELSE false END AS is_mine
	`

	QueryGetAds = `
        SELECT a.id, a.title, a.text, a.image_url, a.price, a.user_id, a.created_at,
               u.login,
               CASE WHEN a.user_id = $1 THEN true ELSE false END AS is_mine
        FROM ads a
        JOIN users u ON a.user_id = u.id
        WHERE a.price >= $2 AND a.price <= $3
        ORDER BY a.%s %s
        LIMIT $4 OFFSET $5
    `

	QueryGetUserById = `
        SELECT id, login, created_at
        FROM users
        WHERE id = $1
    `

	CreateDb = `
        CREATE TABLE IF NOT EXISTS users (
            id SERIAL PRIMARY KEY,
            login VARCHAR(20) UNIQUE NOT NULL,
            password VARCHAR(255) NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
        CREATE TABLE IF NOT EXISTS ads (
            id SERIAL PRIMARY KEY,
            title VARCHAR(100) NOT NULL,
            text TEXT NOT NULL,
            image_url VARCHAR(200) NOT NULL,
            price BIGINT NOT NULL,
            user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
        CREATE INDEX IF NOT EXISTS idx_ads_user_id ON ads(user_id);
        CREATE INDEX IF NOT EXISTS idx_ads_created_at ON ads(created_at);
        CREATE INDEX IF NOT EXISTS idx_ads_price ON ads(price);
    `
)
