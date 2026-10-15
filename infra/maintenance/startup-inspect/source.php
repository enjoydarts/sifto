<?php
// Fixed PUBLIC source extraction only; no application bootstrap or private IO.
declare(strict_types=1);
$files = [
    'composer.json', 'vendor/composer/ClassLoader.php', 'vendor/composer/autoload_real.php',
    'vendor/composer/autoload_psr4.php', 'vendor/composer/autoload_static.php',
    'vendor/symfony/process/Process.php', 'vendor/laravel/horizon/src/WorkerProcess.php',
    'vendor/laravel/horizon/src/Supervisor.php', 'vendor/laravel/horizon/src/ProcessPool.php',
    'vendor/laravel/horizon/src/RedisHorizonCommandQueue.php',
    'vendor/laravel/horizon/src/Repositories/RedisSupervisorRepository.php',
    'vendor/laravel/horizon/src/Repositories/RedisMasterSupervisorRepository.php',
    'vendor/laravel/horizon/src/MasterSupervisor.php',
    'vendor/laravel/horizon/src/Console/SupervisorCommand.php',
    'vendor/laravel/horizon/src/WorkerCommandString.php',
    'vendor/laravel/horizon/src/SupervisorCommandString.php',
    'vendor/laravel/horizon/src/QueueCommandString.php',
    'vendor/laravel/horizon/src/ListensForSignals.php',
    'vendor/laravel/horizon/src/EventMap.php',
    'vendor/laravel/horizon/src/Listeners/PruneTerminatingProcesses.php',
];
foreach ($files as $file) {
    $bytes = file_get_contents('/var/www/html/'.$file);
    if (!is_string($bytes) || strlen($bytes) > 2097152) { exit(1); }
    echo json_encode(['file' => $file, 'sha256' => hash('sha256', $bytes), 'source' => $bytes], JSON_THROW_ON_ERROR)."\n";
}
