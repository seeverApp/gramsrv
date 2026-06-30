-- name: UpsertTempAuthKeyBinding :exec
INSERT INTO temp_auth_key_bindings (
  temp_auth_key_id, perm_auth_key_id, nonce, temp_session_id, expires_at, encrypted_message
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (temp_auth_key_id) DO UPDATE SET
  perm_auth_key_id = EXCLUDED.perm_auth_key_id,
  nonce = EXCLUDED.nonce,
  temp_session_id = EXCLUDED.temp_session_id,
  expires_at = EXCLUDED.expires_at,
  encrypted_message = EXCLUDED.encrypted_message,
  created_at = now();

-- name: GetTempAuthKeyBinding :one
SELECT
  temp_auth_key_id,
  perm_auth_key_id,
  nonce,
  temp_session_id,
  expires_at,
  encrypted_message
FROM temp_auth_key_bindings
WHERE temp_auth_key_id = $1;

-- name: DeleteExpiredTempAuthKeys :execrows
DELETE FROM auth_keys
WHERE auth_key_id IN (
  SELECT temp_auth_key_id
  FROM temp_auth_key_bindings
  WHERE expires_at < $1
  LIMIT $2
);
