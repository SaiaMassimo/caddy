# BinomialHash con Memento - Esempio di Configurazione

## Panoramica

BinomialHash con Memento fornisce consistent hashing che è stabile contro le rimozioni casuali di nodi. Questo significa che quando un upstream diventa unavailable, solo le chiavi che erano mappate a quel nodo vengono ridistribuite, minimizzando l'impatto sui client.

## Configurazione Caddyfile

### Configurazione Base con Consistent Hashing

```caddyfile
# Esempio base con consistent hashing abilitato
reverse_proxy 127.0.0.1:8080 127.0.0.1:8081 127.0.0.1:8082 {
    lb_policy binomial ip {
        consistent
        fallback random
    }
}
```

### Configurazione con URI Hashing

```caddyfile
# Usa l'URI per il hashing invece dell'IP
reverse_proxy 127.0.0.1:8080 127.0.0.1:8081 127.0.0.1:8082 {
    lb_policy binomial uri {
        consistent
        fallback random
    }
}
```

### Configurazione con Header Hashing

```caddyfile
# Usa un header specifico per il hashing
reverse_proxy 127.0.0.1:8080 127.0.0.1:8081 127.0.0.1:8082 {
    lb_policy binomial header {
        field header
        header_field User-Agent
        consistent
        fallback random
    }
}
```

## Configurazione JSON

### Configurazione Base

```json
{
  "reverse_proxy": {
    "upstreams": [
      "127.0.0.1:8080",
      "127.0.0.1:8081", 
      "127.0.0.1:8082"
    ],
    "load_balancing": {
      "selection_policy": {
        "policy": "binomial",
        "field": "ip",
        "consistent": true,
        "fallback": {
          "policy": "random"
        }
      }
    }
  }
}
```

### Configurazione Avanzata

```json
{
  "reverse_proxy": {
    "upstreams": [
      "127.0.0.1:8080",
      "127.0.0.1:8081",
      "127.0.0.1:8082"
    ],
    "load_balancing": {
      "selection_policy": {
        "policy": "binomial",
        "field": "uri",
        "consistent": true,
        "fallback": {
          "policy": "first"
        }
      }
    },
    "health_checks": {
      "active": {
        "path": "/health",
        "interval": "30s",
        "timeout": "5s"
      },
      "passive": {
        "max_fails": 3,
        "fail_duration": "30s"
      }
    }
  }
}
```

## Vantaggi del Consistent Hashing con Memento

### 1. **Minimal Redistribution**
- Solo le chiavi mappate ai nodi rimossi vengono ridistribuite
- Le altre chiavi mantengono la stessa destinazione
- Riduce l'impatto sui client durante i cambiamenti di topologia

### 2. **Stabilità contro Rimozioni Casuali**
- Memento traccia le rimozioni e i loro sostituti
- Permette il ripristino dei nodi nella posizione originale
- Mantiene la consistenza anche con rimozioni multiple

### 3. **Performance Ottimizzate**
- BinomialHash: ~58 ns/op per selezione
- Memento: O(1) lookup per i sostituti
- Thread-safe con mutex ottimizzati

## Esempi di Scenari d'Uso

### Scenario 1: Deployment Rolling
```caddyfile
# Durante un deployment rolling, i nodi vengono temporaneamente rimossi
# e poi riaggiunti. Con consistent hashing, l'impatto è minimo.

reverse_proxy app1:8080 app2:8080 app3:8080 {
    lb_policy binomial ip {
        consistent
    }
    
    health_checks {
        active {
            path /health
            interval 10s
        }
    }
}
```

### Scenario 2: Auto-scaling
```caddyfile
# Con auto-scaling, i nodi vengono aggiunti/rimossi dinamicamente.
# Consistent hashing mantiene la stabilità.

reverse_proxy {
    dynamic srv {
        name _http._tcp.example.com
    }
    
    lb_policy binomial ip {
        consistent
        fallback random
    }
}
```

### Scenario 3: Session Affinity
```caddyfile
# Per applicazioni che richiedono session affinity basata su IP

reverse_proxy 127.0.0.1:8080 127.0.0.1:8081 127.0.0.1:8082 {
    lb_policy binomial ip {
        consistent
    }
    
    # Headers per mantenere informazioni di sessione
    header_up X-Forwarded-For {remote_host}
    header_up X-Real-IP {remote_host}
}
```

## Confronto con Algoritmi Alternativi

| Algoritmo | Redistribution | Performance | Stabilità |
|-----------|----------------|-------------|-----------|
| **BinomialHash + Memento** | Minima | ~58 ns/op | Alta |
| Rendezvous Hashing | Completa | ~205 ns/op | Media |
| Round Robin | Completa | ~50 ns/op | Bassa |
| Random | Completa | ~30 ns/op | Bassa |

## Monitoraggio e Debug

### Log di Debug
```caddyfile
{
    admin {
        listen localhost:2019
    }
    
    logging {
        logs {
            default {
                level DEBUG
            }
        }
    }
}
```

### Metriche
Le metriche di Memento sono disponibili tramite:
- `memento_size`: Numero di nodi rimossi tracciati
- `memento_capacity`: Capacità della tabella di lookup
- `topology_size`: Numero di nodi nella topologia corrente

## Best Practices

### 1. **Abilita Health Checks**
```caddyfile
health_checks {
    active {
        path /health
        interval 30s
        timeout 5s
    }
    passive {
        max_fails 3
        fail_duration 30s
    }
}
```

### 2. **Usa Fallback Appropriato**
```caddyfile
lb_policy binomial ip {
    consistent
    fallback random  # o first, least_conn, etc.
}
```

### 3. **Monitora le Performance**
- Usa i benchmark per misurare le performance
- Monitora la redistribuzione durante i cambiamenti
- Verifica la consistenza delle mappature

### 4. **Configurazione per Ambiente**
```caddyfile
# Sviluppo: consistent hashing disabilitato per semplicità
reverse_proxy localhost:8080 localhost:8081 {
    lb_policy binomial ip {
        # consistent  # Commentato per sviluppo
    }
}

# Produzione: consistent hashing abilitato
reverse_proxy prod1:8080 prod2:8080 prod3:8080 {
    lb_policy binomial ip {
        consistent
        fallback random
    }
}
```

## Troubleshooting

### Problema: Inconsistent Mapping
**Soluzione**: Verifica che `consistent` sia abilitato e che gli upstream siano correttamente configurati.

### Problema: Performance Degradate
**Soluzione**: 
- Verifica che non ci siano troppi nodi rimossi in Memento
- Considera di riavviare Caddy per resettare lo stato
- Monitora le metriche di Memento

### Problema: Nodi Non Ripristinati
**Soluzione**: 
- Verifica che gli health checks funzionino correttamente
- Controlla i log per errori di connessione
- Assicurati che i nodi siano effettivamente disponibili

## Conclusione

BinomialHash con Memento fornisce una soluzione robusta per consistent hashing in Caddy, offrendo:

- **Minimal redistribution** durante i cambiamenti di topologia
- **Alte performance** con ~58 ns/op
- **Stabilità** contro rimozioni casuali di nodi
- **Facile configurazione** tramite Caddyfile o JSON
- **Compatibilità** con tutte le funzionalità esistenti di Caddy

Questa implementazione è ideale per ambienti di produzione che richiedono alta disponibilità e consistenza delle mappature client-upstream.
