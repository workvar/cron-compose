/** Strip CSI/OSC escape sequences and bare CRs from log chunks for <pre> display. */
export function stripAnsi(input: string): string {
  return input
    .replace(/\u001b\[[0-9;?]*[a-zA-Z@]/g, "")
    .replace(/\u001b\][^\u0007]*(?:\u0007|\u001b\\)/g, "")
    .replace(/\u001b./g, "")
    .replace(/\r/g, "");
}
