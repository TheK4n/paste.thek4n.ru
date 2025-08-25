function openTab(evt, tabName) {
    var i, tabcontent, tablinks;
    tabcontent = document.getElementsByClassName("tabcontent");
    for (i = 0; i < tabcontent.length; i++) {
        tabcontent[i].style.display = "none";
    }
    tablinks = document.getElementsByClassName("tablinks");
    for (i = 0; i < tablinks.length; i++) {
        tablinks[i].className = tablinks[i].className.replace(" active", "");
    }
    document.getElementById(tabName).style.display = "block";
    evt.currentTarget.className += " active";
}

function tryEndpoint(baseURL, method, path, endpointId) {
    const responseArea = document.getElementById(`response-${endpointId}`);
    responseArea.innerHTML = '<p>Sending request...</p>';

    const paramsContainer = document.getElementById(`params-${endpointId}`);
    let queryParams = {};
    let pathParams = {};
    let bodyParams = null;
    let headers = {
        'Content-Type': 'application/json'
    };

    if (paramsContainer) {
        const paramInputs = paramsContainer.querySelectorAll('.param-input');

        paramInputs.forEach(inputDiv => {
            const label = inputDiv.querySelector('label');
            const input = inputDiv.querySelector('input, select, textarea');

            if (!label || !input) return;

            const labelText = label.textContent;
            const paramName = labelText.split(' ')[0];
            const paramIn = labelText.match(/\((.*?)\)/)[1];
            const paramValue = input.value;

            if (paramIn === 'query') {
                queryParams[paramName] = paramValue;
            } else if (paramIn === 'path') {
                if (!paramValue) {
                    responseArea.innerHTML = `<p style="color: red;">Error: path parameter cannot be empty</p>`;
                    return
                }
                pathParams[paramName] = paramValue;
            } else if (paramIn === 'body') {
                try {
                    bodyParams = JSON.parse(paramValue);
                } catch (e) {
                    bodyParams = paramValue;
                }
            } else if (paramIn === 'header') {
                headers[paramName] = paramValue;
            }
        });
    }

    let finalPath = path;
    for (const [key, value] of Object.entries(pathParams)) {
        finalPath = finalPath.replace(`{${key}}`, value);
    }

    let url = `${baseURL}${finalPath}`;
    const queryString = Object.keys(queryParams)
        .map(key => `${encodeURIComponent(key)}=${encodeURIComponent(queryParams[key])}`)
        .join('&');

    if (queryString) {
        url += (url.includes('?') ? '&' : '?') + queryString;
    }

    const options = {
        method: method,
        headers: headers
    };

    if (bodyParams !== null && ['POST', 'PUT', 'PATCH'].includes(method.toUpperCase())) {
        if (typeof bodyParams === 'object') {
            options.body = JSON.stringify(bodyParams);
        } else {
            options.body = bodyParams;
            if (!headers['Content-Type']) {
                options.headers['Content-Type'] = 'text/plain';
            }
        }
    }

    fetch(url, options)
        .then(response => {
            const contentType = response.headers.get('Content-Type');
            if (contentType?.includes('application/json')) {
                return response.json();
            } else {
                return response.text();
            }
        })
        .then(data => {
            if (typeof data === 'object') {
                responseArea.innerHTML = `<pre>${JSON.stringify(data, null, 2)}</pre>`;
            } else {
                responseArea.innerHTML = `<pre>${data}</pre>`;
            }
        })
        .catch(error => {
            responseArea.innerHTML = `<p style="color: red;">Error: ${error.message}</p>`;
        });
}
