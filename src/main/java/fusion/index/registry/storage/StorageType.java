package fusion.index.registry.storage;

import fusion.index.registry.enums.StorageBackend;
import jakarta.inject.Qualifier;

import java.lang.annotation.*;

@Qualifier
@Retention(RetentionPolicy.RUNTIME)
@Target({ElementType.TYPE, ElementType.METHOD, ElementType.FIELD, ElementType.PARAMETER})
public @interface StorageType {
    StorageBackend value();
}
