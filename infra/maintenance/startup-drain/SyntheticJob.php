<?php
// PUBLIC only, never installed by the product. No URLs, subprocesses or private inputs.
declare(strict_types=1);
namespace App\SiftoMaintenance;

final class SyntheticJob implements \Illuminate\Contracts\Queue\ShouldQueue
{
    public function __construct(public bool $long = false) {}

    public function handle(): void
    {
        $redis = app('redis')->connection();
        $redis->set('public-drain:started', (string)getmypid());
        $end = hrtime(true) + ($this->long ? 12000000000 : 7000000000);
        do { usleep(100000); } while (hrtime(true) < $end);
        $redis->set('public-drain:completed', (string)getmypid());
    }

    // Fixed PUBLIC read-only parent observation, including after producer expiry.
    public function process($supervisor, $options): void
    {
        if (get_class($supervisor) === \Laravel\Horizon\MasterSupervisor::class && $options === []) {
            // Fixed PUBLIC foreign-state fault. No propagation to child processes.
            $supervisor->working = true;
            file_put_contents('/tmp/public-drain-foreign-master', 'PUBLIC changed'); return;
        }
        if (get_class($supervisor) !== \Laravel\Horizon\Supervisor::class || $options !== []) { return; }
        $s = json_decode(file_get_contents('/var/www/html/storage/app/startup-horizon-drain.state'), true)['data'];
        $same = spl_object_id($supervisor) === $s['original']['supervisor']['object'];
        $codes = []; $signals = []; $paused = $supervisor->working === false;
        foreach ($supervisor->processPools as $i => $pool) {
            $original = $s['original']['supervisor']['pools'][$i];
            $same = $same && spl_object_id($pool) === $original['object'] && $pool->terminatingProcesses === []
                && $pool->options->toArray() === $original['options']
                && array_keys($pool->processes) === array_keys($original['workers']);
            $paused = $paused && $pool->working === false;
            foreach ($pool->processes as $slot => $worker) {
                $same = $same && spl_object_id($worker) === $original['workers'][$slot]['object']
                    && spl_object_id($worker->process) === $original['workers'][$slot]['process_object'];
                $codes[] = $worker->process->getExitCode(); $signals[] = $worker->process->hasBeenSignaled();
            }
        }
        file_put_contents('/tmp/public-drain-exits.json', json_encode(['same_objects' => $same,
            'exit_codes' => $codes, 'signaled' => $signals, 'paused' => $paused], JSON_THROW_ON_ERROR));
    }
}
