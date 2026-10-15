<?php
// PUBLIC fault fixture: executes the exact production tool block, without app bootstrap.
declare(strict_types=1);
$case = $argv[1] ?? '';
if (!in_array($case, ['deadline', 'overflow'], true)) { exit(2); }
$source = file_get_contents('/probe/startup.php');
$gateStart = strpos($source, 'function startupGate(');
$gateEnd = strpos($source, "\nfunction startupResult(");
$runtime = strpos($source, 'function startupRuntime(');
$start = strpos($source, '    $commands = [', $runtime);
$end = strpos($source, "\n    \$startupCategory = 'runtime_process_invalid'; \$after", $start);
if ($gateStart === false || $gateEnd === false || $start === false || $end === false) { exit(2); }
$gate = substr($source, $gateStart, $gateEnd - $gateStart);
$block = substr($source, $start, $end - $start);
eval($gate);
file_put_contents('/tmp/observer-tool-case', $case);
$started = hrtime(true);
// Fixture-only budget; production remains 30 seconds. Child naturally exits after 6s.
$startupDeadline = $started + ($case === 'deadline' ? 2000000000 : 30000000000);
$caught = false;
try { eval($block); }
catch (RuntimeException $failure) { $caught = $failure->getMessage() === 'startup_gate'; }
$elapsed = (hrtime(true) - $started) / 1000000000;
$record = @file_get_contents('/tmp/observer-tool-stat');
$pid = $record === false ? 0 : (int) strstr($record, ' ', true);
// This child is reaped only once /proc no longer contains its PID. No signals here.
$reaped = $pid > 0 && !file_exists('/proc/'.$pid);
$pass = $caught && $reaped && $elapsed < 4.5;
echo json_encode(['case' => $case, 'source_sha256' => hash('sha256', $source),
    'block_sha256' => hash('sha256', $block), 'gate_sha256' => hash('sha256', $gate),
    'elapsed_seconds' => $elapsed, 'gate_exception' => $caught,
    'child_started' => $pid > 0, 'child_reaped' => $reaped, 'pass' => $pass], JSON_THROW_ON_ERROR)."\n";
exit($pass ? 0 : 1);
