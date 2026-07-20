#!/bin/sh
# Inject sidebar CSS and JS into all doc2go-generated HTML files
# Usage: inject-sidebar.sh [site-dir]
SITE_DIR="${1:-site}"

find "$SITE_DIR" -name "*.html" | while read f; do
  sed -i 's|</head>|<link rel="stylesheet" href="/sidebar.css"></head>|' "$f"
  sed -i 's|</body>|<script src="/sidebar.js"></script></body>|' "$f"
done
cp docs/sidebar.css "$SITE_DIR/sidebar.css"
cp docs/sidebar.js "$SITE_DIR/sidebar.js"
