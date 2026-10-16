<?php
// PUBLIC owner observation fixture, never installed as a product resource.
declare(strict_types=1);
namespace App\SiftoMaintenance;
final class RestoreFixture
{
    public function process($actor, $options): void
    {
        if (!in_array($options, [['mode' => 'mixed'], ['mode' => 'paused'], ['mode' => 'read'], ['mode' => 'start']], true)) { return; }
        if (get_class($actor) === \Laravel\Horizon\MasterSupervisor::class) {
            if ($options['mode'] === 'start') {
                $maintenance = app(\Illuminate\Contracts\Foundation\MaintenanceMode::class);
                $maintenance->deactivate();
                app('queue')->connection('redis')->push(new SyntheticJob, '', 'public-drain');
                $r = app('redis')->connection(); $end = hrtime(true) + 10000000000;
                do { if ($r->get('public-drain:started')) { break; } usleep(100000); } while (hrtime(true) < $end);
                $maintenance->activate(['except' => ['/api/health'], 'redirect' => null, 'retry' => null,
                    'refresh' => null, 'secret' => null, 'status' => 503, 'template' => null]);
                $r->rpush('queues:public-drain', 'PUBLIC PENDING A', 'PUBLIC PENDING B');
                $r->zadd('queues:public-drain:delayed', 2000000001, 'PUBLIC DELAYED A', 2000000002, 'PUBLIC DELAYED B');
                $r->zadd('queues:public-drain:reserved', 2000000003, 'PUBLIC RESERVED A', 2000000004, 'PUBLIC RESERVED B');
            }
            if ($options['mode'] === 'paused') { $actor->pause(); }
            file_put_contents('/tmp/public-restore-master.json', json_encode(['working' => $actor->working,
                'count' => count($actor->supervisors), 'live' => $actor->supervisors->first()->process->isRunning()], JSON_THROW_ON_ERROR)); return;
        }
        if (get_class($actor) !== \Laravel\Horizon\Supervisor::class) { return; }
        if ($options['mode'] === 'mixed') { $actor->processPools[1]->pause(); }
        $pools = []; $s = is_file('/var/www/html/storage/app/startup-horizon-drain.state')
            ? json_decode(file_get_contents('/var/www/html/storage/app/startup-horizon-drain.state'), true)['data'] : null;
        foreach ($actor->processPools as $i => $pool) {
            $workers = [];
            foreach ($pool->processes as $slot => $worker) {
                $workers[] = ['worker' => spl_object_id($worker), 'process' => spl_object_id($worker->process),
                    'age_seconds' => microtime(true) - $worker->process->getStartTime(),
                    'live' => $worker->process->isRunning(), 'pid' => $worker->process->getPid(),
                    'native' => $worker->process->getCommandLine() === $pool->options->toWorkerCommand(),
                    'original_native' => $s !== null && spl_object_id($worker->process) === $s['original']['supervisor']['pools'][$i]['workers'][$slot]['process_object']];
            }
            $pools[] = ['working' => $pool->working, 'object' => spl_object_id($pool), 'terminating' => count($pool->terminatingProcesses),
                'options_sha256' => hash('sha256', json_encode($pool->options->toArray(), JSON_THROW_ON_ERROR)), 'workers' => $workers];
        }
        file_put_contents('/tmp/public-restore-owner.json', json_encode(['working' => $actor->working,
            'object' => spl_object_id($actor), 'pools' => $pools], JSON_THROW_ON_ERROR));
    }
}
