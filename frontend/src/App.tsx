import type { Component } from 'solid-js';
import { createSignal, createResource, Match, Switch } from 'solid-js';

async function shortenUrl(url: string, disposable: number, ttl: string): Promise<string> {
  if (disposable > 255) {
    throw new Error("Error: disposable counter cant be more then 255");
  }

  const apiUrl = import.meta.env.VITE_API_URL
  const response = await fetch(`${apiUrl}/?url=true&ttl=${ttl}&disposable=${disposable}`, {
    method: "POST",
    mode: "cors",
    headers: {
      'Content-Type': 'text/plain',
      Accept: 'text/plain',
      'Access-Control-Request-Method': "POST",
      'Access-Control-Request-Headers': "content-type,accept",
    },
    body: url,
  });

  if (!response.ok) {
    throw new Error(`Error! status: ${response.status}`);
  }

  return await response.text();
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text).then(() => {
    alert("Copied");
  });
}

const App: Component = () => {
  const [disposableCounter, setDisposableCounter] = createSignal<number>(0);
  const [expirationTime, setExpirationTime] = createSignal<string>("");
  const [url, setURL] = createSignal<string>("");

  const [shortenedURL, { mutate: _, refetch }] = createResource<string, string>(
    async () => {
      if (!url()) return "";

      const fullUrl = /^https?:\/\//i.test(url()) ? url() : `https://${url()}`;
      return await shortenUrl(fullUrl, disposableCounter(), expirationTime());
    }
  );

  const handleShorten = () => {
    refetch();
  };

  return (
    <div>
      <header>
        <div class="mb-6">
          <label for="url" class="block mb-2 text-sm font-medium text-gray-900 dark:text-white">URL</label>
          <input
            type="url"
            id="url"
            placeholder="https://example.com/"
            value={url()}
            onChange={(e) => setURL(e.currentTarget.value)}
            onKeyPress={(e) => {
              if (e.key === "Enter") {
                setURL(e.currentTarget.value);
                handleShorten();
              }
            }}
            required
            class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
          />
        </div>
        <div class="grid gap-6 mb-6 md:grid-cols-2">
          <div>
            <label for="ttl" class="block mb-2 text-sm font-medium text-gray-900 dark:text-white">URL Expiration Time</label>
            <input
              type="text"
              id="ttl"
              placeholder="24h"
              value={expirationTime()}
              onChange={(e) => setExpirationTime(e.currentTarget.value)}
              onKeyPress={(e) => {
                if (e.key === "Enter") {
                  setExpirationTime(e.currentTarget.value);
                  handleShorten();
                }
              }}
              required
              class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
            />
          </div>
          <div>
            <label for="visitors" class="block mb-2 text-sm font-medium text-gray-900 dark:text-white">Disposable counter</label>
            <input
              type="number"
              id="visitors"
              value={disposableCounter()}
              onInput={(e) => setDisposableCounter(Number(e.currentTarget.value))}
              onKeyPress={(e) => {
                if (e.key === "Enter") {
                  setDisposableCounter(Number(e.currentTarget.value));
                  handleShorten();
                }
              }}
              min="0"
              max="255"
              placeholder="0"
              required
              class="bg-gray-50 border border-gray-300 text-gray-900 text-sm rounded-lg focus:ring-blue-500 focus:border-blue-500 block w-full p-2.5 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
            />
          </div>
        </div>
        <button
          onclick={handleShorten}
          class="text-white bg-blue-700 hover:bg-blue-800 focus:ring-4 focus:outline-none focus:ring-blue-300 font-medium rounded-lg text-sm w-full sm:w-auto px-5 py-2.5 text-center dark:bg-blue-600 dark:hover:bg-blue-700 dark:focus:ring-blue-800"
        >
          Shorten
        </button>

        <Switch>
          <Match when={shortenedURL.loading}>
            <div class="mt-10 text-white">Loading...</div>
          </Match>
          <Match when={shortenedURL.error}>
            <div class="mt-10 text-red-500">Error: {shortenedURL.error.message}</div>
          </Match>
          <Match when={shortenedURL()}>
            <div class="mt-10">
              <div class="flex text-white items-center border border-gray-700 rounded-lg overflow-hidden">
                <input
                  type="text"
                  value={shortenedURL()}
                  readonly
                  class="flex-grow px-4 py-2 outline-none bg-gray-800"
                />
                <button
                  class="bg-blue-500 text-white px-4 py-2 hover:bg-blue-600 transition-colors"
                  onclick={() => copyToClipboard(shortenedURL()!)}
                >
                  Копировать
                </button>
              </div>
            </div>
          </Match>
        </Switch>
      </header>
    </div>
  );
};

export default App;
