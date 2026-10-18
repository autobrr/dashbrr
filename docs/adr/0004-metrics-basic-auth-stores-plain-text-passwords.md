# Metrics basic auth stores plain-text passwords

The metrics listener reads `basic_auth_users` as `user:password` pairs in plain text, the same as qui. autobrr stores bcrypt hashes. We did not follow autobrr, because a hash gives little protection in dashbrr. The same config file and database already hold the API key of every service in plain text. The Prometheus configuration also holds the metrics password in plain text. Plain text also needs no hash tool. Users who copy a qui setup get the same format. A change to bcrypt later is a breaking change for each user who set a password.
