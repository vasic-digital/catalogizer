say() { echo hi; }
work() {
  kill -USR1 $$
  echo worked
}
fail_it() {
  false
}
