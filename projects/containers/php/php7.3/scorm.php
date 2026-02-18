    //Code sample for php/wordpress.

    require_once THEME_LIBS_DIR . DIRECTORY_SEPARATOR . 'autoload.php';
    $ServiceUrl = 'https://cloud.scorm.com/EngineWebServices';
    $AppId = 'xxxxxxx';
    $SecretKey = 'xxxxxxx';
    $Origin = ScormEngineUtilities::getCanonicalOriginString('Healthcare Business Insights', 'Website', '1.0');
    $courseid = get_field('course_id');//just a string
    $ScormService = new ScormEngineService($ServiceUrl, $AppId, $SecretKey, $Origin);
    $regService = $ScormService->getRegistrationService();
    $allResults = $regService->GetRegistrationList($courseid, $email);
    if (!empty($allResults)) {
        $launchUrl = $regService->GetLaunchUrl($allResults[0]->getRegistrationId(), add_query_arg('closeFrame', true, get_permalink())/* some thak-you page */);
        wp_redirect($launchUrl, 302);
        exit();
    } else {
        $registrationId = md5($courseid . '-' . $email);
        $res = $regService->CreateRegistration($registrationId, $courseid, $email, $firstName, $lastName, $email);
        $launchUrl = $regService->GetLaunchUrl($registrationId, add_query_arg('closeFrame', true, get_permalink())/* some thak-you page */);
        wp_redirect($launchUrl, 302);
        exit();
    }
