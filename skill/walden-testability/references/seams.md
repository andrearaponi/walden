# Seams Reference

One entry per ambient source a criterion can depend on. Try the controls in the order the skill gives: the platform control first, with no design change; then one seam carrying the role; a restructuring last. No gap, no change: an entry here is a remedy for a reported gap, never a reason to add a seam.

Every code block is an example of one language's idiom, labeled with that language; the rules live in the skill. Check the version the repository uses before proposing a platform control: the entries name the version that introduced a control where it matters.

Run the search hints inside the design's footprint only, and read every hit before reporting it: a hit is a lead, not a finding.

## Clock

- **Platform control.** Go 1.25 and later: `testing/synctest` runs a test in a bubble whose fake clock starts at midnight UTC 2000-01-01 and advances only when every goroutine in the bubble is durably blocked; `time.Now`, `time.Sleep` and timers inside the bubble use it. Go 1.24 had it only under `GOEXPERIMENT=synctest`, with a different API; earlier versions have nothing equivalent. Vitest: `vi.useFakeTimers()` with `vi.setSystemTime(...)` and `vi.advanceTimersByTime(ms)`. Jest: `jest.useFakeTimers()` with `jest.setSystemTime(...)` and `jest.advanceTimersByTime(ms)`. Both fake `Date` as well as the timers. Node's own test runner: `mock.timers.enable({ apis: ['Date', 'setTimeout'] })`, stable since Node 23.1. .NET: `FakeTimeProvider`, from the package Microsoft.Extensions.TimeProvider.Testing, drives code that already takes a `TimeProvider`, through `SetUtcNow` and `Advance`. Java and Python have no process-wide fake clock: Java gives the seam type instead, `java.time.Clock`, with `Clock.fixed` and `Clock.offset`; Python's `unittest.mock.patch` can replace a module's reference to `time.time` or its imported `datetime` name, but not a method of the built-in `datetime` type.
- **Seam role.** A clock: something that answers "now", passed in where the decision is made. Prefer the platform's own type where one exists, `java.time.Clock` or `InstantSource` in Java and `TimeProvider` in .NET 8 and later; elsewhere a function value or a one-method interface.
- **Search hints.** Go: `time.Now(`, `time.Since(`, `time.After(`, `time.NewTimer(`. TypeScript and JavaScript: `Date.now(`, `new Date(`, `setTimeout(`, `setInterval(`. Python: `datetime.now(`, `date.today(`, `time.time(`, `time.sleep(`. Java: `Instant.now(`, `LocalDate.now(`, `LocalDateTime.now(`, `System.currentTimeMillis(`. C#: `DateTime.Now`, `DateTime.UtcNow`, `DateTimeOffset.UtcNow`, `Task.Delay(`.

**Go 1.25**, no seam: the proof drives the real `time.Now` through the bubble's fake clock.

```go
func TestOverdueAfterDueDate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		loan := Loan{DueAt: time.Now().Add(24 * time.Hour)}
		time.Sleep(25 * time.Hour) // returns at once: the fake clock advances
		if !IsOverdue(loan) {
			t.Fatal("loan not overdue an hour after its due date")
		}
	})
}
```

**Go**, before 1.25 or when the instant is an input of the decision: a function value passed in.

```go
type Service struct {
	now func() time.Time // time.Now in production, a fixed instant in proofs
}

func (s *Service) IsOverdue(l Loan) bool {
	return s.now().After(l.DueAt)
}
```

**Java**, the platform's clock type passed in.

```java
final class LoanService {
    private final Clock clock; // Clock.systemUTC() in production, Clock.fixed(...) in proofs

    LoanService(Clock clock) {
        this.clock = clock;
    }

    boolean isOverdue(Loan loan) {
        return LocalDate.now(clock).isAfter(loan.dueDate());
    }
}
```

**C#**, the platform's time provider passed in.

```csharp
public sealed class LoanService
{
    private readonly TimeProvider _time; // TimeProvider.System in production, FakeTimeProvider in proofs

    public LoanService(TimeProvider time) => _time = time;

    public bool IsOverdue(Loan loan) => _time.GetUtcNow() > loan.DueAt;
}
```

**TypeScript**, no seam: the proof drives `Date` with fake timers.

```ts
vi.useFakeTimers();
vi.setSystemTime(new Date("2026-03-02T00:00:00Z"));
expect(isOverdue({ dueAt: new Date("2026-03-01T00:00:00Z") })).toBe(true);
vi.useRealTimers();
```

**Python**, a callable passed in.

```python
class LoanService:
    def __init__(self, now=datetime.now):  # a fixed lambda in proofs
        self._now = now

    def is_overdue(self, loan):
        return self._now() > loan.due_at
```

## Randomness

- **Platform control.** A fixed seed makes a pseudo-random source repeatable, but only when the code uses a source the proof can seed: Go's `math/rand/v2` gives `rand.New(rand.NewPCG(seed1, seed2))`; Java `new Random(seed)` or any `RandomGenerator`; .NET `new Random(seed)`; Python `random.Random(seed)`. Vitest and Jest can replace `Math.random` for one test with `vi.spyOn(Math, "random")` or `jest.spyOn(Math, "random")`. Shared sources, such as the top-level functions of Go's `math/rand/v2`, .NET's `Random.Shared` or Python's module-level `random.random`, cannot be seeded per test, and cryptographic sources are not meant to be: those need the seam.
- **Seam role.** A random source passed in, or the random value itself passed in, such as an identifier or a token, so that the decision is deterministic.
- **Search hints.** Go: `rand.`, `crypto/rand`, `uuid`. TypeScript and JavaScript: `Math.random(`, `crypto.randomUUID(`, `getRandomValues(`. Python: `random.`, `uuid4(`, `secrets.`. Java: `new Random(`, `ThreadLocalRandom`, `UUID.randomUUID(`. C#: `new Random(`, `Random.Shared`, `Guid.NewGuid(`.

**Go**, a seeded source passed in.

```go
type Drawer struct{ rand *rand.Rand }

func NewDrawer(r *rand.Rand) *Drawer { return &Drawer{rand: r} }

// In a proof: NewDrawer(rand.New(rand.NewPCG(1, 2)))
```

**Java**, the platform interface passed in.

```java
final class Drawer {
    private final RandomGenerator random; // new Random(42) in proofs

    Drawer(RandomGenerator random) {
        this.random = random;
    }
}
```

## Environment

- **Platform control.** Go: `t.Setenv(key, value)` restores the variable after the test, and cannot be used in parallel tests. Vitest: `vi.stubEnv(name, value)`, restored by `vi.unstubAllEnvs()`. Jest: `jest.replaceProperty(process.env, name, value)` for a variable that already exists, restored by `jest.restoreAllMocks()`. Python: `unittest.mock.patch.dict(os.environ, {...})`. .NET: `Environment.SetEnvironmentVariable` changes the whole process, so the proof must restore it and cannot run in parallel. Java has no supported way to change the process environment: it needs the seam.
- **Seam role.** A configuration value: read the environment once at the edge of the program and pass the values in; the decision never reads the environment itself.
- **Search hints.** Go: `os.Getenv(`, `os.LookupEnv(`. TypeScript and JavaScript: `process.env.`, `import.meta.env`. Python: `os.environ`, `os.getenv(`. Java: `System.getenv(`, `System.getProperty(`. C#: `Environment.GetEnvironmentVariable(`.

**Go**, the value read at the edge and passed in.

```go
type Config struct{ SMTPAddr string }

// LoadConfig is the only reader of the environment.
func LoadConfig() Config { return Config{SMTPAddr: os.Getenv("SMTP_ADDR")} }
```

**Java**, a configuration record built once at the edge.

```java
record MailConfig(String host, int port) {
    static MailConfig fromEnvironment() {
        return new MailConfig(System.getenv("SMTP_HOST"), Integer.parseInt(System.getenv("SMTP_PORT")));
    }
}
```

## Network

- **Platform control.** A local server on a loopback port, when the code takes the address as input: Go `net/http/httptest.NewServer`, or `net.Listen("tcp", "127.0.0.1:0")` for another protocol; Java `com.sun.net.httpserver.HttpServer` on port 0; Node `http.createServer().listen(0)`; Python `http.server` in a thread. .NET's `HttpClient` accepts an `HttpMessageHandler` in its constructor, so a proof's handler answers without a socket. Vitest's `vi.mock` and Jest's `jest.mock` replace a whole module: that controls the call without a design change, but couples the proof to the module path.
- **Seam role.** A port: a small interface owned by the application, carrying only the operation the criterion needs, with one adapter for the real protocol. When the protocol itself is the contract, a configurable address instead, so that a proof can point it at a local server.
- **Search hints.** Go: `http.Get(`, `http.DefaultClient`, `net.Dial(`, `smtp.SendMail(`. TypeScript and JavaScript: `fetch(`, `http.request(`, `new WebSocket(`. Python: `urllib.request`, `http.client`, `smtplib.`. Java: `HttpClient.newHttpClient(`, `HttpURLConnection`, `new Socket(`. C#: `new HttpClient(`, `SmtpClient`.

**Go**, a mail port; the production adapter wraps `net/smtp`.

```go
type Mailer interface {
	Send(to, subject, body string) error
}

type Reminders struct{ mail Mailer }
```

**Go**, or the address as configuration, with a loopback listener in the proof.

```go
ln, err := net.Listen("tcp", "127.0.0.1:0") // a fake SMTP server the proof reads
if err != nil {
	t.Fatal(err)
}
cfg := Config{SMTPAddr: ln.Addr().String()}
```

**TypeScript**, the fetch function passed in.

```ts
export function createRatesClient(fetchFn: typeof fetch = fetch) {
  return async (day: string) => (await fetchFn(`/rates/${day}`)).json();
}
```

## Filesystem

- **Platform control.** A temporary directory per test, when the code takes the path as input: Go `t.TempDir()`; Java `Files.createTempDirectory`; .NET `Directory.CreateTempSubdirectory`; Python `tempfile.TemporaryDirectory()`; Node `fs.mkdtemp`. Go's `testing/fstest.MapFS` is an in-memory file system for code that reads through `fs.FS`. Go 1.24 and later: `t.Chdir` for code that resolves relative paths, not in parallel tests.
- **Seam role.** A root directory or a file-system interface passed in, such as Go's `fs.FS` or a Java `Path` resolved against an injected root; never a hard-coded absolute path, the working directory or the home directory.
- **Search hints.** Go: `os.Open(`, `os.ReadFile(`, `os.WriteFile(`, `os.UserHomeDir(`, `os.Getwd(`. TypeScript and JavaScript: `fs.`, `readFileSync(`, `os.homedir(`. Python: `open(`, `Path(`, `Path.home(`. Java: `Files.`, `new File(`, `Paths.get(`, `Path.of(`. C#: `File.`, `Directory.`, `Environment.GetFolderPath(`.

**Go**, reads through `fs.FS`.

```go
// os.DirFS(dir) in production; fstest.MapFS{"rates.json": {Data: data}} in proofs.
func LoadRates(fsys fs.FS) ([]byte, error) {
	return fs.ReadFile(fsys, "rates.json")
}
```

**Python**, a root directory passed in.

```python
def export_statement(loan, root: Path) -> Path:
    path = root / f"{loan.id}.txt"  # a tempfile.TemporaryDirectory() in proofs
    path.write_text(render(loan))
    return path
```

## Concurrency

- **Platform control.** Go 1.25 and later: `synctest.Wait` returns once every other goroutine in the bubble is durably blocked, so a proof sees a background goroutine's effect without sleeping; run proofs with `-race` as well. .NET: code that waits through `Task.Delay(TimeSpan, TimeProvider)` or a timer from `TimeProvider.CreateTimer` advances with `FakeTimeProvider.Advance`. Vitest and Jest: `vi.runAllTimersAsync()` and `jest.runAllTimersAsync()` run the timers and what they schedule. Java and Python have no virtual scheduler in the platform: they need the seam.
- **Seam role.** An executor or scheduler passed in, a synchronous one in proofs, or a completion signal the work returns: a channel, a future, a promise. Never a sleep in the proof.
- **Search hints.** Go: `go func`, `time.Sleep(`, `sync.WaitGroup`, `time.AfterFunc(`. TypeScript and JavaScript: `setTimeout(`, `setInterval(`, `queueMicrotask(`, `new Worker(`. Python: `threading.Thread(`, `asyncio.create_task(`, `ThreadPoolExecutor(`. Java: `new Thread(`, `Executors.`, `CompletableFuture.supplyAsync(`. C#: `Task.Run(`, `new Thread(`, `Parallel.`.

**Go**, the work returns its completion instead of leaving a goroutine nobody can wait for.

```go
func (w *Worker) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.run(ctx)
	}()
	return done
}
```

**Java**, an executor passed in.

```java
final class ReminderScheduler {
    private final Executor executor; // Runnable::run in proofs: the work runs on the caller's thread

    ReminderScheduler(Executor executor) {
        this.executor = executor;
    }

    void schedule(Runnable reminder) {
        executor.execute(reminder);
    }
}
```

## Global State

- **Platform control.** Replacing a global for one test and restoring it: Go swaps a package variable and restores it with `t.Cleanup`, never in parallel tests; Python `unittest.mock.patch.object(module, "NAME", value)`; Vitest `vi.stubGlobal(name, value)` and `vi.resetModules()`; Jest `jest.replaceProperty(object, key, value)` and `jest.resetModules()`, where a module imported again gets fresh module state. Java static fields and .NET static members have no platform restore: they need the seam.
- **Seam role.** The state as a value owned by an instance and passed in through a constructor or an argument, instead of a package variable, a singleton or a static field.
- **Search hints.** Go: package-level `var` written outside `init`, `sync.Once`. TypeScript and JavaScript: module-level `let`, `globalThis.`, an exported singleton instance. Python: `global `, module-level mutable values. Java: non-final `static` fields, `getInstance(`. C#: mutable `static` fields, `.Instance`.

**Go**, a package variable moved into the value that uses it.

```go
// Before: var lateFeePerDay = 50, changed by tests.
type Policy struct{ LateFeePerDay int }

func (p Policy) Fee(daysLate int) int { return p.LateFeePerDay * daysLate }
```

**TypeScript**, module state moved into a factory.

```ts
// Before: let attempts = 0 at module level.
export function createCounter() {
  let attempts = 0;
  return { record: () => ++attempts, attempts: () => attempts };
}
```

## External Service

- **Platform control.** A local fake of the service's protocol started by the proof: a test server on a loopback port, as under Network, or the service itself in a container. A container proof is checkpoint class, never inner loop.
- **Seam role.** A port owned by the application, shaped by what the criterion needs, with one adapter for the real service. Inner-loop proofs use an in-memory implementation of the port; a contract proof pins the adapter against the provider's documented behavior or a recorded fixture of its responses.
- **Search hints.** Client constructors, base URLs and credentials of the service, and its calls inside domain code. Go: `NewClient(`, `sql.Open(`. TypeScript and JavaScript: `Client(` constructors, `fetch(`. Python: `connect(`, `Client(`. Java: `DriverManager.getConnection(`, `HttpClient`. C#: `new SqlConnection(`, `HttpClient`.

**Go**, a payments port and the in-memory implementation an inner-loop proof uses.

```go
type Payments interface {
	Charge(ctx context.Context, loanID string, cents int64) (receipt string, err error)
}

type fakePayments struct{ charged []int64 }

func (f *fakePayments) Charge(_ context.Context, _ string, cents int64) (string, error) {
	f.charged = append(f.charged, cents)
	return "receipt-1", nil
}
```

**Java**, the same port.

```java
interface Payments {
    Receipt charge(String loanId, long cents);
}
```

## Observability: Results And Events

An observability gap closes with the cheapest rung the skill's ladder offers: return the result the criterion names, or emit it through a port a proof can read. A read-only accessor on state that already exists comes before an event; a log is never the remedy unless the log is the response.

**Go**, the attempt count returned instead of kept only in an unexported field.

```go
type SendResult struct {
	Attempts int
	Err      error
}

func (r *Reminders) Send(l Loan) SendResult
```

**TypeScript**, the outcome emitted to a listener passed in.

```ts
export function createReminders(onSent: (loanId: string, attempts: number) => void) {
  // ... calls onSent(loan.id, attempts) once the reminder is delivered
}
```
