(function () {
    'use strict';

    const sidebar = document.getElementById('sidebar');
    const detail = document.getElementById('detail');
    const statusBadge = document.getElementById('status');
    const emptyList = document.getElementById('empty-list');

    let requests = [];
    let selectedId = null;

    // --- WebSocket realtime ---

    function connectWS() {
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = new WebSocket(proto + '//' + location.host + '/ws');

        ws.onopen = function () {
            statusBadge.textContent = 'Connected';
            statusBadge.className = 'badge connected';
        };

        ws.onmessage = function (e) {
            try {
                const rec = JSON.parse(e.data);
                // Update existing or prepend
                const idx = requests.findIndex(r => r.ID === rec.ID);
                if (idx >= 0) {
                    requests[idx] = rec;
                } else {
                    requests.unshift(rec);
                }
                renderSidebar();
                if (selectedId === rec.ID) {
                    renderDetail(rec);
                }
            } catch (err) {
                console.error('ws parse:', err);
            }
        };

        ws.onclose = function () {
            statusBadge.textContent = 'Disconnected';
            statusBadge.className = 'badge disconnected';
            setTimeout(connectWS, 2000);
        };

        ws.onerror = function () {
            ws.close();
        };
    }

    // --- Fetch initial data ---

    function fetchRequests() {
        fetch('/api/requests')
            .then(r => r.json())
            .then(data => {
                requests = data || [];
                renderSidebar();
            })
            .catch(err => console.error('fetch:', err));
    }

    // --- Render ---

    function renderSidebar() {
        if (requests.length === 0) {
            emptyList.style.display = 'flex';
            return;
        }
        emptyList.style.display = 'none';

        // Remove old items but keep emptyList element
        const items = sidebar.querySelectorAll('.request-item');
        items.forEach(el => el.remove());

        requests.forEach(rec => {
            const el = document.createElement('div');
            el.className = 'request-item' + (rec.ID === selectedId ? ' active' : '');
            el.onclick = function () {
                selectedId = rec.ID;
                renderSidebar();
                renderDetail(rec);
            };

            const method = document.createElement('span');
            method.className = 'method ' + rec.Method;
            method.textContent = rec.Method;

            const path = document.createElement('span');
            path.className = 'req-path';
            path.textContent = rec.Path;

            const meta = document.createElement('span');
            meta.className = 'req-meta';

            if (rec.StatusCode) {
                const badge = document.createElement('span');
                const cls = rec.StatusCode < 300 ? 's2xx' : rec.StatusCode < 400 ? 's3xx' : rec.StatusCode < 500 ? 's4xx' : 's5xx';
                badge.className = 'status-badge ' + cls;
                badge.textContent = rec.StatusCode;
                meta.appendChild(badge);
            }

            if (rec.CreatedAt) {
                const time = document.createElement('span');
                time.className = 'req-time';
                time.textContent = formatTime(rec.CreatedAt);
                meta.appendChild(time);
            }

            if (rec.DurationMs) {
                const dur = document.createElement('span');
                dur.className = 'req-time';
                dur.textContent = rec.DurationMs + 'ms';
                meta.appendChild(dur);
            }

            el.appendChild(method);
            el.appendChild(path);
            el.appendChild(meta);
            sidebar.appendChild(el);
        });
    }

    function renderDetail(rec) {
        let html = '';

        // Summary
        html += '<div class="detail-summary">';
        html += '<span class="method ' + rec.Method + '">' + rec.Method + '</span>';
        html += '<span class="path">' + escapeHtml(rec.Path) + '</span>';
        if (rec.StatusCode) {
            const cls = rec.StatusCode < 300 ? 's2xx' : rec.StatusCode < 400 ? 's3xx' : rec.StatusCode < 500 ? 's4xx' : 's5xx';
            html += '<span class="status-badge ' + cls + '">' + rec.StatusCode + '</span>';
        }
        if (rec.DurationMs) {
            html += '<span class="req-time">' + rec.DurationMs + 'ms</span>';
        }
        html += '</div>';

        // Request headers
        html += '<div class="section">';
        html += '<h2>Request Headers</h2>';
        html += renderHeaders(rec.Headers);
        html += '</div>';

        // Request body
        if (rec.Body) {
            html += '<div class="section">';
            html += '<h2>Request Body <button class="copy-btn" onclick="copyText(this)">Copy</button></h2>';
            html += '<pre>' + formatBody(rec.Body) + '</pre>';
            html += '</div>';
        }

        // Response headers
        if (rec.ResHeaders) {
            html += '<div class="section">';
            html += '<h2>Response Headers</h2>';
            html += renderHeaders(rec.ResHeaders);
            html += '</div>';
        }

        // Response body
        if (rec.ResBody) {
            html += '<div class="section">';
            html += '<h2>Response Body <button class="copy-btn" onclick="copyText(this)">Copy</button></h2>';
            html += '<pre>' + formatBody(rec.ResBody) + '</pre>';
            html += '</div>';
        }

        // Metadata
        html += '<div class="section">';
        html += '<h2>Metadata</h2>';
        html += '<div class="header-row"><span class="header-key">ID:</span><span class="header-val">' + rec.ID + '</span></div>';
        html += '<div class="header-row"><span class="header-key">Tunnel:</span><span class="header-val">' + rec.TunnelID + '</span></div>';
        if (rec.CreatedAt) {
            html += '<div class="header-row"><span class="header-key">Time:</span><span class="header-val">' + rec.CreatedAt + '</span></div>';
        }
        html += '</div>';

        detail.innerHTML = html;
    }

    function renderHeaders(headersStr) {
        if (!headersStr || headersStr === '{}') return '<div class="header-row"><span class="header-val">(none)</span></div>';
        try {
            const headers = typeof headersStr === 'string' ? JSON.parse(headersStr) : headersStr;
            let html = '';
            for (const [key, vals] of Object.entries(headers)) {
                const val = Array.isArray(vals) ? vals.join(', ') : vals;
                html += '<div class="header-row"><span class="header-key">' + escapeHtml(key) + ':</span><span class="header-val">' + escapeHtml(val) + '</span></div>';
            }
            return html;
        } catch (e) {
            return '<pre>' + escapeHtml(headersStr) + '</pre>';
        }
    }

    function formatBody(body) {
        if (!body) return '';
        // body might be base64 encoded (from Go []byte JSON marshaling)
        let str = body;
        if (typeof body === 'string' && isBase64(body)) {
            try {
                str = atob(body);
            } catch (e) {
                str = body;
            }
        }
        // Try to pretty-print JSON
        try {
            const obj = JSON.parse(str);
            return escapeHtml(JSON.stringify(obj, null, 2));
        } catch (e) {
            return escapeHtml(str);
        }
    }

    function isBase64(str) {
        if (str.length % 4 !== 0) return false;
        return /^[A-Za-z0-9+/]*={0,2}$/.test(str);
    }

    function formatTime(ts) {
        try {
            const d = new Date(ts);
            return d.toLocaleTimeString();
        } catch (e) {
            return ts;
        }
    }

    function escapeHtml(str) {
        if (typeof str !== 'string') return '';
        return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }

    // Global copy helper
    window.copyText = function (btn) {
        const pre = btn.parentElement.nextElementSibling;
        if (pre) {
            navigator.clipboard.writeText(pre.textContent).then(() => {
                btn.textContent = 'Copied!';
                setTimeout(() => { btn.textContent = 'Copy'; }, 1500);
            });
        }
    };

    // --- Init ---
    fetchRequests();
    connectWS();
})();
