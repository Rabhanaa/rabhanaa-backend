-- name: CreateUserDocument :one
INSERT INTO user_documents (user_id, document_type, file_path)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, document_type) DO UPDATE SET file_path = $3, uploaded_at = NOW()
RETURNING *;

-- name: GetUserDocuments :many
SELECT * FROM user_documents WHERE user_id = $1;

-- name: CountUserDocuments :one
SELECT COUNT(*) FROM user_documents WHERE user_id = $1;

-- name: GetMissingDocumentTypes :many
SELECT unnest(ARRAY['business_license','national_id','tax_card'])
EXCEPT
SELECT document_type FROM user_documents WHERE user_id = $1;
