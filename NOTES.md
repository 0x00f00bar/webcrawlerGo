The `href` attribute of an `<a>` tag can contain a variety of values, which can be categorized as follows:

1. **Absolute URLs**:
   - Full URLs that include the scheme (e.g., `http://`, `https://`, `ftp://`, etc.).
   - Example: `<a href="https://www.example.com/page">Link</a>`

2. **Relative URLs**:
   - URLs relative to the current page's URL.
   - Example: `<a href="/about">About</a>` (absolute path)
   - Example: `<a href="page.html">Page</a>` (relative to the current directory)

3. **Fragment Identifiers**:
   - Links that point to a specific part of the current page.
   - Example: `<a href="#section1">Section 1</a>`

4. **Mailto Links**:
   - Links that open the user's email client.
   - Example: `<a href="mailto:someone@example.com">Email</a>`

5. **Tel Links**:
   - Links for telephone numbers, often used in mobile devices.
   - Example: `<a href="tel:+1234567890">Call Us</a>`

6. **JavaScript Links**:
   - Links that execute JavaScript code.
   - Example: `<a href="javascript:void(0)">Click me</a>`

7. **Data URLs**:
   - URLs that embed small data items directly within the link.
   - Example: `<a href="data:text/plain;base64,SGVsbG8sIFdvcmxkIQ==">Data Link</a>`

8. **File Protocol Links**:
   - Links that point to local files, usually not accessible over the web.
   - Example: `<a href="file:///C:/path/to/file.txt">Local File</a>`

9. **Special Protocols**:
   - Links using protocols specific to certain applications or services.
   - Example: `<a href="ftp://ftp.example.com/file.zip">FTP Link</a>`

10. **Empty Links**:
    - Links without an `href` value, which may not be functional.
    - Example: `<a href="">No destination</a>`

11. **Link with JavaScript events**:
    - Links that use event attributes (e.g., `onclick`).
    - Example: `<a href="#" onclick="alert('Hello')">Alert</a>`

When developing or analyzing web crawlers, it's crucial to handle these different types appropriately, as they can impact how links are followed and indexed.

SQL to get latest pages for is_monitored urls
WITH LatestPages AS (
    SELECT u.url, p.id, p.added_at, p.url_id,
           ROW_NUMBER() OVER (PARTITION BY u.id ORDER BY p.added_at DESC) AS rn
    FROM pages p
    JOIN urls u ON p.url_id = u.id
	WHERE u.is_monitored=true and u.url like 'base-url%'
	and u.url like '%sub-url-pattern%'
	and p.added_at <= '2024-10-03 24:00:00'
)
SELECT *
FROM LatestPages
WHERE rn = 1
LIMIT page_size OFFSET (page_number - 1) * page_size;
