// Quotes a table/column name for the given connection type:
// "name" for PostgreSQL, `name` for MySQL and SQLite.
export const quoteIdentifier = (identifier, dbType = 'mysql') => {
  const value = String(identifier || '')
  if (dbType === 'postgres') {
    return `"${value.replace(/"/g, '""')}"`
  }

  return `\`${value.replace(/`/g, '``')}\``
}
