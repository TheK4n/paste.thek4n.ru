import { Show } from "solid-js";

type CustomAlertProps = {
  message: string;
  isVisible: boolean;
  onClose: () => void;
};

export function CustomAlert(props: CustomAlertProps) {
  return (
    <Show when={props.isVisible}>
      <div class="fixed inset-x-0 top-20 flex items-center justify-center z-50">
        <div class="bg-white p-6 rounded-lg shadow-lg max-w-sm dark:bg-gray-800 w-full">
          <div class="flex justify-between items-center mb-4">
            <h3 class="text-lg font-medium text-white">Copied</h3>
            <button
              onclick={props.onClose}
              class="text-gray-500 hover:text-gray-700"
            >
              ✕
            </button>
          </div>
          <p class="text-white">{props.message}</p>
        </div>
      </div>
    </Show>
  );
}
